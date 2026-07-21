-- 004_preset_base_url.sql
-- Add base_url and default_model to provider_presets for one-click provider setup

ALTER TABLE provider_presets ADD COLUMN base_url TEXT DEFAULT '';
ALTER TABLE provider_presets ADD COLUMN default_model TEXT DEFAULT '';

-- Update existing presets with correct API base URLs
UPDATE provider_presets SET base_url = 'https://api.openai.com/v1',          default_model = 'gpt-4o'                              WHERE name = 'OpenAI';
UPDATE provider_presets SET base_url = 'https://api.anthropic.com/v1',       default_model = 'claude-sonnet-4-20250514'             WHERE name = 'Anthropic';
UPDATE provider_presets SET base_url = 'https://generativelanguage.googleapis.com/v1beta', default_model = 'gemini-2.5-pro'   WHERE name = 'Google AI';
UPDATE provider_presets SET base_url = 'https://api.deepseek.com/v1',        default_model = 'deepseek-chat'                        WHERE name = 'DeepSeek';
UPDATE provider_presets SET base_url = 'https://api.groq.com/openai/v1',     default_model = 'llama-3.3-70b-versatile'              WHERE name = 'Groq';
UPDATE provider_presets SET base_url = 'https://api.together.xyz/v1',        default_model = 'meta-llama/Llama-3.3-70B-Instruct-Turbo' WHERE name = 'Together AI';
UPDATE provider_presets SET base_url = 'https://api.fireworks.ai/inference/v1', default_model = 'accounts/fireworks/models/llama-v3p1-70b-instruct' WHERE name = 'Fireworks AI';
UPDATE provider_presets SET base_url = 'https://openrouter.ai/api/v1',       default_model = 'openai/gpt-4o'                        WHERE name = 'OpenRouter';
UPDATE provider_presets SET base_url = 'https://api.mistral.ai/v1',          default_model = 'mistral-large-latest'                 WHERE name = 'Mistral AI';
UPDATE provider_presets SET base_url = 'https://api.cohere.ai/v1',           default_model = 'command-r-plus'                       WHERE name = 'Cohere';
UPDATE provider_presets SET base_url = 'https://api.perplexity.ai',          default_model = 'sonar-pro'                            WHERE name = 'Perplexity';
UPDATE provider_presets SET base_url = 'https://api.x.ai/v1',                default_model = 'grok-2'                               WHERE name = 'xAI (Grok)';
UPDATE provider_presets SET base_url = 'https://api.llama-api.com',          default_model = 'llama3.1-70b'                         WHERE name = 'Meta Llama';
UPDATE provider_presets SET base_url = 'https://beecode.cc/v1',              default_model = 'gpt-4o-mini'                          WHERE name = 'BeeCode';
UPDATE provider_presets SET base_url = 'http://127.0.0.1:15721/v1',          default_model = 'gpt-5.6-sol'                          WHERE name = 'cc-switch';
