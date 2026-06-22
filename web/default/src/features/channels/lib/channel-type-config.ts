/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { CHANNEL_TYPES } from '../constants'

// ============================================================================
// Channel Type Configuration
// ============================================================================

export interface ChannelTypeConfig {
  id: number
  name: string
  icon: string
  defaultBaseUrl?: string
  requiresOrganization?: boolean
  requiresRegion?: boolean
  supportedModels?: string[]
  hints?: {
    baseUrl?: string
    key?: string
    models?: string
    other?: string
  }
  validation?: {
    keyFormat?: RegExp
    keyMinLength?: number
  }
}

/**
 * Configuration for each channel type
 */
export const CHANNEL_TYPE_CONFIGS: Record<number, ChannelTypeConfig> = {
  1: {
    id: 1,
    name: CHANNEL_TYPES[1],
    icon: 'openai',
    defaultBaseUrl: 'https://api.openai.com',
    requiresOrganization: true,
    hints: {
      baseUrl: 'Default: https://api.openai.com',
      key: 'Format: sk-...',
      models: 'gpt-4,gpt-4-turbo,gpt-3.5-turbo',
    },
    validation: {
      keyFormat: /^sk-/,
      keyMinLength: 20,
    },
  },
  3: {
    id: 3,
    name: CHANNEL_TYPES[3],
    icon: 'azure',
    requiresRegion: true,
    hints: {
      baseUrl: 'Azure OpenAI Endpoint',
      key: 'Azure API Key',
      models: 'Deployment names',
    },
  },
  14: {
    id: 14,
    name: CHANNEL_TYPES[14],
    icon: 'anthropic',
    defaultBaseUrl: 'https://api.anthropic.com',
    hints: {
      key: 'Format: sk-ant-...',
      models: 'claude-3-opus,claude-3-sonnet,claude-3-haiku',
    },
  },
  24: {
    id: 24,
    name: CHANNEL_TYPES[24],
    icon: 'google',
    hints: {
      key: 'Google API Key',
      models: 'gemini-pro,gemini-pro-vision',
    },
  },
  41: {
    id: 41,
    name: CHANNEL_TYPES[41],
    icon: 'google',
    requiresRegion: true,
    hints: {
      key: 'Service account JSON or API key',
      models: 'gemini-pro,gemini-1.5-pro',
      other: 'Region config: {"default": "us-central1"}',
    },
  },
  43: {
    id: 43,
    name: CHANNEL_TYPES[43],
    icon: 'deepseek',
    defaultBaseUrl: 'https://api.deepseek.com',
    hints: {
      key: 'DeepSeek API Key',
      models: 'deepseek-chat,deepseek-coder',
    },
  },
  20: {
    id: 20,
    name: CHANNEL_TYPES[20],
    icon: 'openrouter',
    defaultBaseUrl: 'https://openrouter.ai/api',
    hints: {
      key: 'OpenRouter API Key',
      models: 'Use model IDs from OpenRouter',
    },
  },
  56: {
    id: 56,
    name: CHANNEL_TYPES[56],
    icon: 'replicate',
    defaultBaseUrl: 'https://api.replicate.com',
    hints: {
      key: 'Replicate API Token',
      models: 'Replicate model IDs',
      baseUrl: 'Default: https://api.replicate.com',
    },
  },
  // Free Tier Providers (from 9router/AIClient2API)
  65: {
    id: 65,
    name: 'Kiro AI (Free)',
    icon: 'kiro',
    defaultBaseUrl: 'https://codewhisperer.us-east-1.amazonaws.com',
    hints: {
      key: 'AWS SSO OAuth Token (use Kiro OAuth flow to obtain)',
      models: 'claude-haiku-4-5,claude-sonnet-4-5,claude-sonnet-4-6,claude-opus-4-5,claude-opus-4-6,claude-opus-4-7,claude-opus-4-8',
    },
  },
  66: {
    id: 66,
    name: 'MiMo Code Free',
    icon: 'mimo',
    defaultBaseUrl: 'https://api.xiaomimimo.com/api/free-ai/openai/chat',
    hints: {
      key: 'No API key needed. Enter "free" as placeholder.',
      models: 'mimo-auto',
    },
  },
  67: {
    id: 67,
    name: 'Qoder',
    icon: 'qoder',
    defaultBaseUrl: 'https://api.qoder.com/v1',
    hints: {
      key: 'OAuth Token (use Connect Qoder button to authorize)',
      models: 'qoder-auto,gpt-4o,claude-sonnet-4',
    },
  },
  68: {
    id: 68,
    name: 'Agnes',
    icon: 'agnes',
    defaultBaseUrl: 'https://apihub.agnes-ai.com',
    hints: {
      key: 'Agnes API Key',
      models: 'agnes-video-v2.0,agnes-video-v2.0-fast',
    },
  },
  69: {
    id: 69,
    name: 'Gemini CLI',
    icon: 'google',
    defaultBaseUrl: 'https://generativelanguage.googleapis.com',
    hints: {
      key: 'Gemini API Key',
      models: 'gemini-2.5-pro,gemini-2.5-flash,gemini-2.0-flash,gemini-1.5-pro,gemini-1.5-flash',
    },
  },
  70: {
    id: 70,
    name: 'Antigravity',
    icon: 'google',
    defaultBaseUrl: 'https://daily-cloudcode-pa.googleapis.com',
    hints: {
      key: 'Google OAuth Token',
      models: 'gemini-3-flash,gemini-3-pro-high,gemini-pro-agent,claude-sonnet-4-6',
    },
  },
  71: {
    id: 71,
    name: 'Grok CLI',
    icon: 'xai',
    defaultBaseUrl: 'https://api.x.ai',
    hints: {
      key: 'xAI OAuth Token',
      models: 'grok-3,grok-3-mini,grok-4,grok-4.1-thinking',
    },
  },
  58: { id: 58, name: 'Tavily', icon: 'tavily', defaultBaseUrl: 'https://api.tavily.com', hints: { key: 'Tavily API Key (tvly-...)' } },
  59: { id: 59, name: 'Brave Search', icon: 'brave', defaultBaseUrl: 'https://api.search.brave.com', hints: { key: 'Brave Search API Key' } },
  60: { id: 60, name: 'Serper', icon: 'serper', defaultBaseUrl: 'https://google.serper.dev', hints: { key: 'Serper API Key' } },
  61: { id: 61, name: 'Exa', icon: 'exa', defaultBaseUrl: 'https://api.exa.ai', hints: { key: 'Exa API Key' } },
  62: { id: 62, name: 'SearXNG', icon: 'searxng', hints: { baseUrl: 'Your SearXNG instance URL' } },
  63: { id: 63, name: 'Jina Reader', icon: 'jina', defaultBaseUrl: 'https://r.jina.ai', hints: { key: 'Jina API Key (optional)' } },
  64: { id: 64, name: 'Firecrawl', icon: 'firecrawl', defaultBaseUrl: 'https://api.firecrawl.dev', hints: { key: 'Firecrawl API Key (fc-...)' } },
  72: {
    id: 72,
    name: 'JoyCode (JD)',
    icon: 'joycode',
    defaultBaseUrl: 'https://joycode.jd.com',
    hints: {
      key: 'OAuth 2.0 Bearer Token',
      models: 'coder-model,vision-model,qwen3-coder-plus,qwen3-coder-flash',
    },
  },
  73: {
    id: 73,
    name: 'Cursor',
    icon: 'cursor',
    defaultBaseUrl: 'https://api2.cursor.sh',
    hints: {
      key: 'Cursor session token',
      models: 'gpt-4o,claude-sonnet-4,claude-opus-4,cursor-small,o3-mini',
    },
  },
  74: {
    id: 74,
    name: 'GitHub Copilot',
    icon: 'github',
    defaultBaseUrl: 'https://api.githubcopilot.com',
    hints: {
      key: 'Copilot token (from VS Code subscription)',
      models: 'gpt-4o,gpt-5,claude-sonnet-4,o4-mini,o1',
    },
  },
  75: {
    id: 75,
    name: 'Xiaomi TokenPlan',
    icon: 'xiaomi',
    defaultBaseUrl: 'https://api.xiaomimimo.com',
    hints: {
      key: 'Bearer token from Xiaomi TokenPlan',
      models: 'claude-sonnet-4,claude-opus-4,claude-haiku-4.5,mimo-v2.5-pro',
    },
  },
  76: {
    id: 76,
    name: 'CommandCode',
    icon: 'commandcode',
    defaultBaseUrl: 'https://api.commandcode.ai',
    hints: {
      key: 'API Key (format: user_xxx)',
      models: 'commandcode-default',
    },
  },
  77: {
    id: 77,
    name: 'ChatGPT2API (Free Web)',
    icon: 'openai',
    defaultBaseUrl: 'http://localhost:3000',
    hints: {
      baseUrl: 'ChatGPT2API sidecar URL (e.g. http://chatgpt2api:80)',
      key: 'Auth key from chatgpt2api config.json',
      models: 'gpt-4o,gpt-5,gpt-image-2,auto (models auto-detected from sidecar)',
      other: 'Deploy chatgpt2api as Docker sidecar: docker compose up -d',
    },
  },
}

/**
 * Get configuration for a channel type
 */
export function getChannelTypeConfig(type: number): ChannelTypeConfig {
  return (
    CHANNEL_TYPE_CONFIGS[type] || {
      id: type,
      name: CHANNEL_TYPES[type as keyof typeof CHANNEL_TYPES] || 'Unknown',
      icon: 'openai',
    }
  )
}

/**
 * Check if channel type requires organization field
 */
export function requiresOrganization(type: number): boolean {
  return CHANNEL_TYPE_CONFIGS[type]?.requiresOrganization || false
}

/**
 * Check if channel type requires region configuration
 */
export function requiresRegion(type: number): boolean {
  return CHANNEL_TYPE_CONFIGS[type]?.requiresRegion || false
}

/**
 * Get default base URL for channel type
 */
export function getDefaultBaseUrl(type: number): string {
  return CHANNEL_TYPE_CONFIGS[type]?.defaultBaseUrl || ''
}

/**
 * Get hints for channel type
 */
export function getChannelTypeHints(type: number) {
  return CHANNEL_TYPE_CONFIGS[type]?.hints || {}
}

/**
 * Validate API key format for channel type
 */
export function validateKeyFormat(type: number, key: string): boolean {
  const config = CHANNEL_TYPE_CONFIGS[type]
  if (!config?.validation) return true

  const { keyFormat, keyMinLength } = config.validation

  if (keyMinLength && key.length < keyMinLength) {
    return false
  }

  if (keyFormat && !keyFormat.test(key)) {
    return false
  }

  return true

}
