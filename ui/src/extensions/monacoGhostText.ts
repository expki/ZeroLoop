import type * as monacoTypes from 'monaco-editor'

type Monaco = typeof monacoTypes

export function registerGhostTextProvider(
  monaco: Monaco,
  completionFn: (prefix: string, suffix: string, filename: string, signal: AbortSignal) => Promise<string>
): monacoTypes.IDisposable {
  let debounceTimer: ReturnType<typeof setTimeout> | null = null
  let currentController: AbortController | null = null

  const provider: monacoTypes.languages.InlineCompletionsProvider = {
    provideInlineCompletions: async (
      model: monacoTypes.editor.ITextModel,
      position: monacoTypes.Position,
      _context: monacoTypes.languages.InlineCompletionContext,
      token: monacoTypes.CancellationToken
    ): Promise<monacoTypes.languages.InlineCompletions> => {
      // Cancel previous request
      if (currentController) {
        currentController.abort()
        currentController = null
      }
      if (debounceTimer) {
        clearTimeout(debounceTimer)
        debounceTimer = null
      }

      // Don't suggest when there's a selection
      // (We can't directly check selection here, but the context will have triggerKind)

      return new Promise((resolve) => {
        debounceTimer = setTimeout(async () => {
          if (token.isCancellationRequested) {
            resolve({ items: [] })
            return
          }

          const controller = new AbortController()
          currentController = controller

          // Link Monaco cancellation to AbortController
          token.onCancellationRequested(() => controller.abort())

          try {
            const lineCount = model.getLineCount()
            let prefix: string
            let suffix: string

            if (lineCount <= 500) {
              // Full file mode
              prefix = model.getValueInRange({
                startLineNumber: 1,
                startColumn: 1,
                endLineNumber: position.lineNumber,
                endColumn: position.column,
              })
              suffix = model.getValueInRange({
                startLineNumber: position.lineNumber,
                startColumn: position.column,
                endLineNumber: lineCount,
                endColumn: model.getLineMaxColumn(lineCount),
              })
            } else {
              // Adaptive window: ~300 lines before, ~200 lines after cursor
              const startLine = Math.max(1, position.lineNumber - 300)
              const endLine = Math.min(lineCount, position.lineNumber + 200)
              prefix = model.getValueInRange({
                startLineNumber: startLine,
                startColumn: 1,
                endLineNumber: position.lineNumber,
                endColumn: position.column,
              })
              suffix = model.getValueInRange({
                startLineNumber: position.lineNumber,
                startColumn: position.column,
                endLineNumber: endLine,
                endColumn: model.getLineMaxColumn(endLine),
              })
            }

            if (!prefix && !suffix) {
              resolve({ items: [] })
              return
            }

            const filename = model.uri.path.split('/').pop() || 'unknown'
            const text = await completionFn(prefix, suffix, filename, controller.signal)

            if (controller.signal.aborted || !text) {
              resolve({ items: [] })
              return
            }

            resolve({
              items: [
                {
                  insertText: text,
                  range: new monaco.Range(
                    position.lineNumber,
                    position.column,
                    position.lineNumber,
                    position.column
                  ),
                },
              ],
            })
          } catch {
            resolve({ items: [] })
          }
        }, 400) // 400ms debounce
      })
    },

    disposeInlineCompletions: () => {
      // Nothing to dispose per-completion
    },
  }

  const disposable = monaco.languages.registerInlineCompletionsProvider(
    { pattern: '**' },
    provider
  )

  // Return a composite disposable that also cleans up timers
  return {
    dispose: () => {
      if (debounceTimer) {
        clearTimeout(debounceTimer)
        debounceTimer = null
      }
      if (currentController) {
        currentController.abort()
        currentController = null
      }
      disposable.dispose()
    },
  }
}
