import React, { useState } from "react";
import { api, AICoachResponse } from "../lib/api";

export const AICoachPage: React.FC = () => {
  const [prompt, setPrompt] = useState("");
  const [loading, setLoading] = useState(false);
  const [response, setResponse] = useState<AICoachResponse | null>(null);

  async function handleAsk(e: React.FormEvent) {
    e.preventDefault();
    if (!prompt.trim()) return;
    setLoading(true);
    try {
      const res = await api.askAICoach(prompt);
      setResponse(res.coach);
    } catch {
      /* ignora erro */
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="max-w-4xl mx-auto p-6 space-y-6">
      <header className="border-b border-gray-200 pb-4">
        <h1 className="text-3xl font-bold text-teal-800">🤖 Coach IA FortalRunners</h1>
        <p className="text-gray-600">Assistente inteligente especializado no clima, terreno e treinos de corrida em Fortaleza.</p>
      </header>

      <form onSubmit={handleAsk} className="flex gap-3">
        <input
          type="text"
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
          placeholder="Ex: Qual o melhor horário para correr na Beira-Mar hoje?"
          className="flex-1 px-4 py-3 border border-gray-300 rounded-xl focus:ring-2 focus:ring-teal-500 focus:outline-none text-sm"
        />
        <button
          type="submit"
          disabled={loading}
          className="bg-teal-700 hover:bg-teal-800 text-white font-bold px-6 py-3 rounded-xl text-sm transition disabled:opacity-50"
        >
          {loading ? "Consultando..." : "Perguntar ao Coach"}
        </button>
      </form>

      {response && (
        <div className="bg-gradient-to-br from-teal-900 to-teal-950 text-white p-6 rounded-2xl border border-teal-700 shadow-xl space-y-4">
          <div className="flex items-center gap-2">
            <span className="text-xl">🏃‍♂️</span>
            <h3 className="font-bold text-teal-200 text-lg">Recomendação Personalizada</h3>
          </div>

          <p className="text-base leading-relaxed text-teal-50">{response.message}</p>

          {response.tips && response.tips.length > 0 && (
            <div className="bg-teal-950/60 p-4 rounded-xl border border-teal-800 space-y-2">
              <h4 className="text-xs font-bold text-amber-300 uppercase tracking-wider">💡 Dicas de Treino & Hidratação</h4>
              <ul className="list-disc list-inside space-y-1 text-sm text-teal-100">
                {response.tips.map((tip, idx) => (
                  <li key={idx}>{tip}</li>
                ))}
              </ul>
            </div>
          )}

          <div className="text-[11px] text-teal-400 border-t border-teal-800/60 pt-3 flex justify-between">
            <span>IA Model: Gemini 2.0 Flash</span>
            <span>{new Date(response.created_at).toLocaleTimeString("pt-BR")}</span>
          </div>
        </div>
      )}
    </div>
  );
};
