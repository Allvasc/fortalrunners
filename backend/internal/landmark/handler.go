package landmark

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/Allvasc/fortalrunners/backend/internal/auth"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/blobstore"
)

type Handler struct {
	svc   *Service
	blobs blobstore.Store
}

func NewHandler(svc *Service, blobs blobstore.Store) *Handler {
	return &Handler{svc: svc, blobs: blobs}
}

func (h *Handler) Register(g *echo.Group) {
	g.GET("/landmarks", h.list)
	g.GET("/landmarks/:id", h.get)
	g.POST("/landmarks/:id/checkin", h.checkin)
	g.GET("/me/landmarks", h.userProgress)
}

func (h *Handler) list(c echo.Context) error {
	userID := auth.UserID(c)
	prog, err := h.svc.GetProgress(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao listar marcos")
	}
	return c.JSON(http.StatusOK, prog)
}

func (h *Handler) get(c echo.Context) error {
	userID := auth.UserID(c)
	lmID := c.Param("id")

	lm, err := h.svc.GetLandmark(c.Request().Context(), lmID, userID)
	if errors.Is(err, ErrLandmarkNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "marco não encontrado")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao buscar marco")
	}

	return c.JSON(http.StatusOK, lm)
}

// checkin aceita multipart (campo `photo` + `lat`/`lng`/`run_id`) ou, sem
// arquivo, JSON com `photo_key` já enviado. A foto é obrigatória (plano §3).
func (h *Handler) checkin(c echo.Context) error {
	userID := auth.UserID(c)
	lmID := c.Param("id")

	var lat, lng float64
	var runID, photoKey *string

	if fh, ferr := c.FormFile("photo"); ferr == nil {
		f, err := fh.Open()
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "não foi possível ler a foto")
		}
		defer f.Close()
		raw, _ := io.ReadAll(io.LimitReader(f, 9<<20))
		key, err := h.blobs.PutImage(c.Request().Context(), "landmark_checkin", userID, "landmark_checkin", raw)
		switch {
		case errors.Is(err, blobstore.ErrBadImage):
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "envie uma foto JPEG ou PNG")
		case errors.Is(err, blobstore.ErrTooLarge):
			return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "foto muito grande")
		case err != nil:
			return echo.NewHTTPError(http.StatusInternalServerError, "erro ao processar a foto")
		}
		photoKey = &key
		lat, _ = strconv.ParseFloat(c.FormValue("lat"), 64)
		lng, _ = strconv.ParseFloat(c.FormValue("lng"), 64)
		if r := c.FormValue("run_id"); r != "" {
			runID = &r
		}
	} else {
		var req struct {
			Lat      float64 `json:"lat"`
			Lng      float64 `json:"lng"`
			RunID    *string `json:"run_id,omitempty"`
			PhotoKey *string `json:"photo_key,omitempty"`
		}
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "requisição inválida")
		}
		lat, lng, runID, photoKey = req.Lat, req.Lng, req.RunID, req.PhotoKey
	}

	chk, err := h.svc.Checkin(c.Request().Context(), userID, lmID, runID, photoKey, lat, lng)
	switch {
	case errors.Is(err, ErrLandmarkNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "marco não encontrado")
	case errors.Is(err, ErrPhotoRequired):
		return echo.NewHTTPError(http.StatusBadRequest, "envie a foto tirada na câmera do app")
	case errors.Is(err, ErrRiskZone):
		return echo.NewHTTPError(http.StatusForbidden, "este marco está numa zona de risco ativa e não aceita check-in agora")
	case errors.Is(err, ErrTooFar):
		return echo.NewHTTPError(http.StatusBadRequest, "você precisa estar mais próximo do marco para fazer check-in")
	case errors.Is(err, ErrAlreadyCheckedIn):
		return echo.NewHTTPError(http.StatusConflict, "você já realizou o check-in neste marco")
	case err != nil:
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao realizar check-in")
	}

	return c.JSON(http.StatusAccepted, chk) // 202: entrou na fila de moderação
}

func (h *Handler) userProgress(c echo.Context) error {
	userID := auth.UserID(c)
	prog, err := h.svc.GetProgress(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao buscar progresso")
	}
	return c.JSON(http.StatusOK, prog)
}
