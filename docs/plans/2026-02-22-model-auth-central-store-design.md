# Model & Authentication Central Store - Design Document

## Overview

Implement a centralized configuration store for AI models and authentication credentials in the Bot Portal. This will allow agents to select from pre-configured models and authentication setups during creation/editing, providing better security, consistency, and management.

## Context

Currently, the Bot Portal manages agent lifecycle (Docker containers) and A2A message routing, but lacks centralized configuration for AI models and authentication. Each agent would need to manage its own credentials and model configuration, leading to:

- Duplicated credential storage across agents
- Inconsistent model configurations
- Security risks from scattered API keys
- Poor operational visibility into what models/auth are being used

## Architecture

### Core Entities

**Models Table:**
- `id` - Unique identifier
- `name` - Display name (e.g., "GPT-4 Turbo", "Claude-3.5 Sonnet")
- `provider` - AI provider (openai, anthropic, azure, custom)
- `model_identifier` - Provider-specific model name
- `endpoint_url` - Provider API endpoint (nullable for standard providers)
- `default_params` - JSON blob for temperature, max_tokens, etc.
- `created_at`, `updated_at`

**Auth Configurations Table:**
- `id` - Unique identifier
- `name` - Display name (e.g., "OpenAI Production", "Anthropic Dev")
- `provider` - Matches model provider
- `auth_type` - (api_key, bearer_token, oauth)
- `credentials` - Encrypted JSON blob containing secrets
- `endpoint_url` - Provider endpoint (nullable)
- `created_at`, `updated_at`

**Agent Updates:**
- Add `model_id` foreign key to agents table
- Add `auth_config_id` foreign key to agents table

### Data Flow

```
Portal Admin → Creates Models & Auth Configs → Central Store
                                                      ↓
Agent Creation/Edit → Selects from dropdowns → Agent references IDs
                                                      ↓
Agent Container Start → Portal injects model/auth → Environment Variables
```

### Security

- **Credential Encryption**: All sensitive auth data encrypted at rest using AES-256
- **Environment Injection**: Portal decrypts and injects credentials as environment variables to agent containers
- **No Agent Storage**: Agents never store credentials locally
- **Audit Trail**: Track which agents use which models/auth configs

### API Design

**Models Management:**
```
GET    /api/models              # List all models
POST   /api/models              # Create model
GET    /api/models/{id}         # Get model details
PUT    /api/models/{id}         # Update model
DELETE /api/models/{id}         # Delete model
```

**Auth Management:**
```
GET    /api/auth-configs        # List configs (masked credentials)
POST   /api/auth-configs        # Create config
GET    /api/auth-configs/{id}   # Get config (masked credentials)
PUT    /api/auth-configs/{id}   # Update config
DELETE /api/auth-configs/{id}   # Delete config
```

**Agent Integration:**
```
PUT    /api/agents/{id}         # Update to include model_id, auth_config_id
```

### UI Components

**Models Page:**
- Table of configured models with provider, endpoint
- Add/Edit modal with form fields
- Test connection button
- Usage count (how many agents use this model)

**Auth Configs Page:**
- Table of auth configs with masked credentials
- Add/Edit modal with secure input fields
- Test auth button
- Usage count

**Agent Creation/Edit:**
- Model dropdown populated from `/api/models`
- Auth Config dropdown populated from `/api/auth-configs`
- Show selected model + auth details in summary

### Migration Strategy

1. **Database Migration**: Add new tables and agent foreign keys
2. **Backward Compatibility**: Existing agents continue working with null model/auth IDs
3. **Default Configs**: Create default model/auth entries for common providers
4. **Gradual Migration**: Update agents to use central store over time

### Implementation Plan

**Phase 1: Data Layer**
- Create database migrations for models and auth_configs tables
- Implement encryption/decryption utilities
- Add CRUD operations for both entities

**Phase 2: API Layer**
- Implement REST endpoints for models and auth configs
- Add validation and security middleware
- Update agent endpoints to handle new fields

**Phase 3: Container Integration**
- Modify Docker manager to inject model/auth environment variables
- Update agent startup to read configuration from environment
- Add credential rotation support

**Phase 4: Frontend**
- Create Models and Auth Configs management pages
- Update agent creation/edit forms with dropdowns
- Add validation and testing capabilities

### Environment Variable Injection

When starting agent containers, portal will inject:
```bash
# Model Configuration
MODEL_PROVIDER=openai
MODEL_NAME=gpt-4-turbo
MODEL_ENDPOINT=https://api.openai.com/v1

# Authentication
AUTH_TYPE=api_key
API_KEY=sk-...
AUTH_ENDPOINT=https://api.openai.com/v1

# Optional Model Parameters
MODEL_TEMPERATURE=0.7
MODEL_MAX_TOKENS=4096
```

### Error Handling

- **Missing References**: Graceful degradation if model/auth config deleted
- **Credential Failures**: Clear error messages without exposing secrets
- **Connection Issues**: Retry logic and fallback mechanisms
- **Validation**: Comprehensive input validation for all endpoints

### Success Criteria

- ✅ Centralized model and authentication management
- ✅ Secure credential storage with encryption
- ✅ Simple agent configuration via dropdowns
- ✅ Backward compatibility with existing agents
- ✅ Audit trail of configuration usage
- ✅ UI for managing models and auth configurations
- ✅ Environment variable injection to agent containers