```
User → StreamRequest → StreamOrchestrator
                           ↓
        ┌──────────────────┼──────────────────┐
        ↓                  ↓                  ↓
   OpenAI Stream    Google Stream    (Future: Batch Stream)
        ↓                  ↓                  ↓
        └──────────────────┼──────────────────┘
                           ↓
                   Unified Event Channel
                           ↓
        ┌──────────────────┼──────────────────┐
        ↓                  ↓                  ↓
    Text Chunks      Tool Requests       Metadata
```