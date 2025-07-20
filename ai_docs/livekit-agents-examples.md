# LiveKit Agents Examples Index

A comprehensive guide to the LiveKit Python Agents examples repository, organized by use case and technical patterns.

## 🎯 Quick Reference by Use Case

### **Phone & Telephony Systems**
- **[answer_call.py](../python-agents-examples/telephony/answer_call.py)** - Basic incoming SIP call handling
- **[make_call/](../python-agents-examples/telephony/make_call/)** - Outbound calling with agent dispatch
- **[warm_handoff.py](../python-agents-examples/telephony/warm_handoff.py)** - Transfer calls from AI to human agents
- **[sip_lifecycle.py](../python-agents-examples/telephony/sip_lifecycle.py)** - Complete SIP call lifecycle management
- **[survey_caller/](../python-agents-examples/telephony/survey_caller/)** - Automated survey calling system

**Best for**: Call centers, customer support, automated surveys, phone assistants

---

### **Healthcare & Medical**
- **[medical_office_triage/](../python-agents-examples/complex-agents/medical_office_triage/)** - Patient triage and routing system
- **[nutrition-assistant/](../python-agents-examples/complex-agents/nutrition-assistant/)** - Health and wellness food logging

**Best for**: Healthcare automation, patient management, wellness applications

---

### **E-commerce & Retail**
- **[personal_shopper/](../python-agents-examples/complex-agents/personal_shopper/)** - E-commerce customer support and sales
- **[shopify-voice-shopper/](../python-agents-examples/complex-agents/shopify-voice-shopper/)** - Voice-enabled shopping integration

**Best for**: Online retail, customer service, sales automation

---

### **Education & Entertainment**
- **[role-playing/](../python-agents-examples/complex-agents/role-playing/)** - Interactive D&D-style RPG system
- **[tavus/](../python-agents-examples/avatars/tavus/)** - Educational AI agent with visual avatars

**Best for**: Educational tools, interactive gaming, tutoring systems

---

### **Documentation & Knowledge Management**
- **[rag/](../python-agents-examples/rag/)** - RAG-enabled agent for document queries
- **[large_context.py](../python-agents-examples/pipeline-llm/large_context.py)** - Handling large document contexts

**Best for**: Customer support, internal documentation, knowledge bases

---

## 🔧 Technical Patterns & Components

### **Foundation Patterns** (`basics/`)
- **[listen_and_respond.py](../python-agents-examples/basics/listen_and_respond.py)** - Core STT→LLM→TTS pipeline
- **[function_calling.py](../python-agents-examples/basics/function_calling.py)** - `@function_tool` decorator usage
- **[context_variables.py](../python-agents-examples/basics/context_variables.py)** - Dynamic prompt templating
- **[uninterruptable.py](../python-agents-examples/basics/uninterruptable.py)** - Agent that continues speaking
- **[playing_audio.py](../python-agents-examples/basics/playing_audio.py)** - Audio file playback

**Best for**: Learning agent development, testing components, basic chatbots

---

### **Speech Processing** (`pipeline-stt/`)
- **[transcriber.py](../python-agents-examples/pipeline-stt/transcriber.py)** - Real-time speech transcription
- **[keyword_detection.py](../python-agents-examples/pipeline-stt/keyword_detection.py)** - Keyword spotting in speech
- **[diarization.py](../python-agents-examples/pipeline-stt/diarization.py)** - Speaker identification and separation

**Best for**: Meeting transcription, voice commands, speaker analytics

---

### **Voice Synthesis** (`pipeline-tts/`)
- **[tts_comparison.py](../python-agents-examples/pipeline-tts/tts_comparison.py)** - Side-by-side TTS provider comparison
- **[elevenlabs_change_language.py](../python-agents-examples/pipeline-tts/elevenlabs_change_language.py)** - Dynamic language switching
- **[cartesia_tts.py](../python-agents-examples/pipeline-tts/cartesia_tts.py)** - Cartesia TTS integration
- **[openai_tts.py](../python-agents-examples/pipeline-tts/openai_tts.py)** - OpenAI TTS integration

**Best for**: Multilingual applications, voice comparison, character voices

---

### **LLM Integration** (`pipeline-llm/`)
- **[anthropic_llm.py](../python-agents-examples/pipeline-llm/anthropic_llm.py)** - Claude integration
- **[ollama_llm.py](../python-agents-examples/pipeline-llm/ollama_llm.py)** - Local LLM deployment
- **[llm_powered_content_filter.py](../python-agents-examples/pipeline-llm/llm_powered_content_filter.py)** - AI-based content moderation
- **[interrupt_user.py](../python-agents-examples/pipeline-llm/interrupt_user.py)** - LLM-controlled interruption

**Best for**: Multi-provider deployments, content moderation, local AI deployment

---

### **Real-time Processing** (`realtime/`)
- **[openai-realtime.py](../python-agents-examples/realtime/openai-realtime.py)** - Basic realtime conversation
- **[openai-realtime-tools.py](../python-agents-examples/realtime/openai-realtime-tools.py)** - Function calling with realtime API
- **[openai-realtime-drive-thru.py](../python-agents-examples/realtime/openai-realtime-drive-thru.py)** - Drive-through ordering scenario

**Best for**: Low-latency applications, streaming conversations, interactive systems

---

### **Workflow Management** (`flows/`)
- **[simple_flow.py](../python-agents-examples/flows/simple_flow.py)** - Linear conversation flows
- **[declarative_flow.py](../python-agents-examples/flows/declarative_flow.py)** - Config-driven conversation paths
- **[multi_stage_flow.py](../python-agents-examples/flows/multi_stage_flow.py)** - Complex branching conversations

**Best for**: Surveys, onboarding processes, complex decision trees

---

### **Visual & Multimodal** (`avatars/`, `complex-agents/vision/`)
- **[hedra/](../python-agents-examples/avatars/hedra/)** - Hedra avatar integration (pipeline, realtime, dynamic)
- **[vision/agent.py](../python-agents-examples/complex-agents/vision/agent.py)** - Multimodal AI with video processing

**Best for**: Customer-facing applications, educational tools, visual AI assistants

---

## 🏗️ Advanced Architecture Patterns

### **Multi-Agent Systems**
- **[medical_office_triage/](../python-agents-examples/complex-agents/medical_office_triage/)** - Agent-to-agent transfers with context preservation
- **[personal_shopper/](../python-agents-examples/complex-agents/personal_shopper/)** - Multi-agent customer service workflow
- **[long_or_short_agent.py](../python-agents-examples/multi-agent/long_or_short_agent.py)** - Context-based agent selection

**Best for**: Complex problem solving, specialized agent teams, enterprise workflows

---

### **State Management & Persistence**
- **[personal_shopper/database.py](../python-agents-examples/complex-agents/personal_shopper/database.py)** - SQLite integration patterns
- **[role-playing/core/game_state.py](../python-agents-examples/complex-agents/role-playing/core/game_state.py)** - Complex state management
- **[tracking_state/npc_character.py](../python-agents-examples/tracking_state/npc_character.py)** - Character state tracking

**Best for**: Applications requiring persistent data, session continuity, complex state

---

### **External Integration**
- **[mcp/](../python-agents-examples/mcp/)** - Model Context Protocol server integration
- **[home_assistant/](../python-agents-examples/home_assistant/)** - IoT device control
- **[rpc/rpc_agent.py](../python-agents-examples/rpc/rpc_agent.py)** - RPC communication with state database

**Best for**: IoT applications, external system integration, protocol-based communication

---

## 🔍 Monitoring & Testing

### **Performance Monitoring** (`metrics/`)
- **[metrics_llm.py](../python-agents-examples/metrics/metrics_llm.py)** - LLM performance tracking
- **[metrics_stt.py](../python-agents-examples/metrics/metrics_stt.py)** - Speech-to-text metrics
- **[metrics_tts.py](../python-agents-examples/metrics/metrics_tts.py)** - Text-to-speech metrics
- **[send-metrics-to-3p/](../python-agents-examples/metrics/send-metrics-to-3p/)** - Third-party metrics integration

**Best for**: Production monitoring, performance optimization, analytics dashboards

---

### **Testing & Evaluation** (`evaluating-agents/`)
- **[agent_evals.py](../python-agents-examples/evaluating-agents/agent_evals.py)** - Agent-to-agent evaluation
- **[agent_to_test.py](../python-agents-examples/evaluating-agents/agent_to_test.py)** - Test agent implementation

**Best for**: Quality assurance, agent performance testing, CI/CD pipelines

---

## 🌐 Specialized Integration Patterns

### **Translation & Internationalization** (`translators/`)
- **[pipeline_translator.py](../python-agents-examples/translators/pipeline_translator.py)** - Pipeline-level translation
- **[tts_translator.py](../python-agents-examples/translators/tts_translator.py)** - Translation with TTS

### **Hardware & Edge Computing** (`hardware/`)
- **[pi-zero-transcriber/](../python-agents-examples/hardware/pi-zero-transcriber/)** - Raspberry Pi deployment

### **Event Systems** (`events/`)
- **[basic_event.py](../python-agents-examples/events/basic_event.py)** - Event handling patterns
- **[event_emitters.py](../python-agents-examples/events/event_emitters.py)** - Custom event emitters

### **Recording & Analysis** (`egress/`)
- **[recording_agent.py](../python-agents-examples/egress/recording_agent.py)** - Session recording patterns

---

## 📋 Frontend Integration Examples

Many examples include complete frontend implementations:

- **React/Next.js**: tavus, nova-sonic, nutrition-assistant, turn-taking
- **JavaScript/HTML**: ivr-agent
- **Browser Extension**: shopify-voice-shopper

**Best for**: Full-stack voice AI applications, web integration patterns

---

## 🚀 Getting Started Recommendations

### **New to LiveKit Agents?**
1. Start with **[basics/listen_and_respond.py](../python-agents-examples/basics/listen_and_respond.py)**
2. Try **[basics/function_calling.py](../python-agents-examples/basics/function_calling.py)** for tool integration
3. Explore **[telephony/answer_call.py](../python-agents-examples/telephony/answer_call.py)** for phone integration

### **Building Production Applications?**
1. Study **[complex-agents/medical_office_triage/](../python-agents-examples/complex-agents/medical_office_triage/)** for multi-agent patterns
2. Review **[metrics/](../python-agents-examples/metrics/)** for monitoring
3. Examine **[evaluating-agents/](../python-agents-examples/evaluating-agents/)** for testing strategies

### **Specific Technology Integration?**
- **RAG/Knowledge Base**: [rag/](../python-agents-examples/rag/)
- **Realtime API**: [realtime/](../python-agents-examples/realtime/)
- **Visual Avatars**: [avatars/](../python-agents-examples/avatars/)
- **IoT/Smart Home**: [home_assistant/](../python-agents-examples/home_assistant/)

---

*Each example includes detailed README files with setup instructions, use cases, and implementation notes. This index provides a high-level overview to help you quickly identify the most relevant examples for your specific needs.*