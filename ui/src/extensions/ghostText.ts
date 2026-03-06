import {
  EditorView,
  ViewPlugin,
  ViewUpdate,
  Decoration,
  WidgetType,
  keymap,
} from '@codemirror/view'
import { StateField, StateEffect, Extension, Prec } from '@codemirror/state'

// --- State Effects ---
const setSuggestion = StateEffect.define<{ text: string; pos: number }>()
const clearSuggestion = StateEffect.define<void>()

// --- Widget ---
class GhostTextWidget extends WidgetType {
  constructor(readonly text: string) {
    super()
  }

  toDOM(): HTMLElement {
    const span = document.createElement('span')
    span.className = 'cm-ghost-text'
    // Cap at 3 lines
    const lines = this.text.split('\n')
    const capped = lines.slice(0, 3)
    if (lines.length > 3) {
      capped[2] = capped[2] + '...'
    }
    span.textContent = capped.join('\n')
    return span
  }

  eq(other: GhostTextWidget): boolean {
    return this.text === other.text
  }

  ignoreEvent(): boolean {
    return true
  }
}

// --- State Field ---
const ghostTextField = StateField.define<{ text: string; pos: number } | null>({
  create() {
    return null
  },
  update(value, tr) {
    for (const effect of tr.effects) {
      if (effect.is(setSuggestion)) return effect.value
      if (effect.is(clearSuggestion)) return null
    }
    // Clear on any document change
    if (tr.docChanged) return null
    return value
  },
})

// --- Decoration ---
const ghostTextDecoration = EditorView.decorations.compute([ghostTextField], (state) => {
  const suggestion = state.field(ghostTextField)
  if (!suggestion) return Decoration.none
  const widget = Decoration.widget({
    widget: new GhostTextWidget(suggestion.text),
    side: 1,
  })
  return Decoration.set([widget.range(suggestion.pos)])
})

// --- ViewPlugin for side effects ---
class GhostTextPlugin {
  private timer: ReturnType<typeof setTimeout> | null = null
  private controller: AbortController | null = null

  constructor(
    private view: EditorView,
    private completionFn: (prefix: string, suffix: string, signal: AbortSignal) => Promise<string>
  ) {}

  update(update: ViewUpdate) {
    if (!update.docChanged) return

    // Note: suggestion is already cleared by the StateField's docChanged handler

    // Abort any in-flight request
    this.controller?.abort()
    this.controller = null

    // Clear previous debounce timer
    if (this.timer !== null) {
      clearTimeout(this.timer)
      this.timer = null
    }

    // Start new debounced request
    this.timer = setTimeout(() => {
      this.requestCompletion()
    }, 400)
  }

  private async requestCompletion() {
    const controller = new AbortController()
    this.controller = controller

    const state = this.view.state
    if (!state.selection.main.empty) return
    const pos = state.selection.main.head
    const doc = state.doc

    // Build prefix and suffix from document, split at cursor
    let prefix: string
    let suffix: string

    if (doc.lines <= 500) {
      // Full file mode
      prefix = doc.sliceString(0, pos)
      suffix = doc.sliceString(pos)
    } else {
      // Adaptive window mode: ~300 lines before, ~200 lines after cursor
      const cursorLine = doc.lineAt(pos)
      const startLine = Math.max(1, cursorLine.number - 300)
      const endLine = Math.min(doc.lines, cursorLine.number + 200)
      const windowStart = doc.line(startLine).from
      const windowEnd = doc.line(endLine).to
      prefix = doc.sliceString(windowStart, pos)
      suffix = doc.sliceString(pos, windowEnd)
    }

    if (!prefix && !suffix) return

    try {
      const text = await this.completionFn(prefix, suffix, controller.signal)
      // Verify the view hasn't been destroyed and cursor hasn't moved
      if (controller.signal.aborted) return
      if (!text) return
      if (this.view.state.selection.main.head !== pos) return

      this.view.dispatch({
        effects: setSuggestion.of({ text, pos }),
      })
    } catch {
      // Silently ignore errors (AbortError, network errors, etc.)
    }
  }

  destroy() {
    if (this.timer !== null) {
      clearTimeout(this.timer)
      this.timer = null
    }
    this.controller?.abort()
    this.controller = null
  }
}

// --- Keymap ---
const ghostTextKeymap = keymap.of([
  {
    key: 'Tab',
    run(view) {
      const suggestion = view.state.field(ghostTextField)
      if (!suggestion || !view.state.selection.main.empty) return false
      view.dispatch({
        changes: { from: suggestion.pos, insert: suggestion.text },
        selection: { anchor: suggestion.pos + suggestion.text.length },
        effects: clearSuggestion.of(undefined),
      })
      return true
    },
  },
  {
    key: 'Escape',
    run(view) {
      const suggestion = view.state.field(ghostTextField)
      if (!suggestion) return false
      view.dispatch({ effects: clearSuggestion.of(undefined) })
      return true
    },
  },
])

// --- Public API ---
export function ghostTextExtension(
  completionFn: (prefix: string, suffix: string, signal: AbortSignal) => Promise<string>
): Extension {
  return [
    ghostTextField,
    ghostTextDecoration,
    ViewPlugin.define((view) => new GhostTextPlugin(view, completionFn)),
    Prec.highest(ghostTextKeymap),
  ]
}
