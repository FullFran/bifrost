import { createAssistantMessageEventStream, getCurrentSystemPrompt } from '@earendil-works/pi-ai';
import type { ExtensionAPI } from '@earendil-works/pi-coding-agent';
import { registerBifrost } from './runtime.mjs';

export default function bifrost(pi: ExtensionAPI) {
  registerBifrost(pi, { createStream: createAssistantMessageEventStream, getCurrentSystemPrompt });
}
