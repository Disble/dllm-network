// PoC: "obtener todos los datos" de una respuesta /api/generate de Ollama.
//
// Objetivo: demostrar que NADA viene encriptado. El body es JSON plano; lo que
// confunde es (a) el campo `response` viene como string JSON-escapado y (b) el
// campo `context` es un array de token IDs del tokenizer (no un cifrado).
//
// Correr:  node poc/decode-ollama-response.mjs   (o: bun poc/decode-ollama-response.mjs)

const RAW = {
  model: 'gemma4:12b',
  created_at: '2026-06-18T14:59:35.9829943Z',
  response:
    '[\n  {"tokens": 1, "ipa": "bʌt"},\n  {"tokens": 1, "ipa": "ˈʌðɚ"},\n  {"tokens": 1, "ipa": "saɪəntɪsts"},\n  {"tokens": 1, "ipa": "θɪŋk"},\n  {"tokens": 2, "ipa": "ðət"},\n  {"tokens": 1, "ipa": "seɪlz"},\n  {"tokens": 2, "ipa": "kʊɾəv"},\n  {"tokens": 2, "ipa": "bɪn juːzd"}\n]',
  done: true,
  done_reason: 'stop',
  // context recortado a una muestra representativa: son token IDs, no un cifrado.
  context: [2, 105, 2364, 107, 3048, 659, 614, 7710, 528, 696, 236772, 3118, 34816],
  total_duration: 12872249400,
  load_duration: 9255609800,
  prompt_eval_count: 381,
  prompt_eval_duration: 337045000,
  eval_count: 142,
  eval_duration: 3189019000,
};

const NS_PER_MS = 1e6;
const NS_PER_S = 1e9;

const ns = (v) => {
  const ms = v / NS_PER_MS;
  return ms >= 1000 ? `${(v / NS_PER_S).toFixed(2)} s` : `${ms.toFixed(1)} ms`;
};

// 1) El `response` ES texto plano: parseable como JSON sin ninguna clave.
let parsedResponse = null;
let responseIsJson = false;
try {
  parsedResponse = JSON.parse(RAW.response);
  responseIsJson = true;
} catch {
  parsedResponse = RAW.response; // texto libre legítimo (no todo `response` es JSON)
}

// 2) El `context` son token IDs: lo resumimos, no lo volcamos crudo.
const contextSummary = {
  tokenCount: RAW.context.length,
  firstTokens: RAW.context.slice(0, 8),
  lastTokens: RAW.context.slice(-8),
  note: 'Conversación tokenizada para continuar el contexto en la próxima request. No es cifrado.',
};

// 3) Telemetría: nanosegundos -> humano + tasa derivada.
const tokensPerSec = RAW.eval_count / (RAW.eval_duration / NS_PER_S);
const timing = {
  total: ns(RAW.total_duration),
  load: ns(RAW.load_duration),
  promptEval: `${ns(RAW.prompt_eval_duration)} · ${RAW.prompt_eval_count} tokens`,
  eval: `${ns(RAW.eval_duration)} · ${RAW.eval_count} tokens`,
  throughput: `${tokensPerSec.toFixed(1)} tok/s`,
};

console.log('=== META ===');
console.log({ model: RAW.model, created_at: RAW.created_at, done: RAW.done, done_reason: RAW.done_reason });

console.log('\n=== RESPONSE (parseado y legible) ===');
console.log('responseIsJson:', responseIsJson);
console.log(responseIsJson ? JSON.stringify(parsedResponse, null, 2) : parsedResponse);

console.log('\n=== CONTEXT (resumido, no crudo) ===');
console.log(contextSummary);

console.log('\n=== TIMING (ns -> humano) ===');
console.log(timing);
