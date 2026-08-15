import * as React from "react"
import { EditorContent, useEditor, type Editor } from "@tiptap/react"
import StarterKit from "@tiptap/starter-kit"
import { TaskItem, TaskList } from "@tiptap/extension-list"
import { Placeholder } from "@tiptap/extensions"
import { Markdown, type MarkdownStorage } from "tiptap-markdown"
import { HugeiconsIcon, type IconSvgElement } from "@hugeicons/react"
import {
  CheckListIcon,
  Heading02Icon,
  Heading03Icon,
  LeftToRightBlockQuoteIcon,
  LeftToRightListBulletIcon,
  LeftToRightListNumberIcon,
  SourceCodeIcon,
  TextBoldIcon,
  TextItalicIcon,
  TextStrikethroughIcon,
} from "@hugeicons/core-free-icons"

import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { useI18n } from "@/lib/i18n/context"
import type { TranslationKey } from "@/lib/i18n/translations"

// Typography for the rendered document. Kept here (instead of globals.css) so
// the editor and the read-only renderer can never drift apart.
const CONTENT_CLASS = [
  "text-sm/relaxed outline-none",
  "[&_p]:my-2 [&_p:first-child]:mt-0 [&_p:last-child]:mb-0",
  "[&_h1]:mt-4 [&_h1]:mb-2 [&_h1]:font-heading [&_h1]:text-lg [&_h1]:font-semibold",
  "[&_h2]:mt-4 [&_h2]:mb-2 [&_h2]:font-heading [&_h2]:text-base [&_h2]:font-semibold",
  "[&_h3]:mt-3 [&_h3]:mb-1.5 [&_h3]:font-heading [&_h3]:text-sm [&_h3]:font-semibold",
  "[&_ul]:my-2 [&_ul]:list-disc [&_ul]:pl-5",
  "[&_ol]:my-2 [&_ol]:list-decimal [&_ol]:pl-5",
  "[&_li]:my-0.5 [&_li>p]:my-0",
  // Task lists render their own checkbox, so they opt out of list markers and
  // the indent the marker would have needed. Items are matched through the
  // list: the task item's node view renders a plain `li` without the
  // `data-type` its renderHTML would have added.
  "[&_ul[data-type=taskList]]:list-none [&_ul[data-type=taskList]]:pl-0",
  "[&_ul[data-type=taskList]>li]:flex [&_ul[data-type=taskList]>li]:items-start [&_ul[data-type=taskList]>li]:gap-2",
  "[&_ul[data-type=taskList]>li>label]:mt-[0.3em] [&_ul[data-type=taskList]>li>label]:flex [&_ul[data-type=taskList]>li>label]:shrink-0",
  // A task item wraps its content in a div, so the `li>p` reset above misses
  // it and the paragraph would keep its block margin — pushing the text a line
  // below the checkbox instead of beside it.
  "[&_ul[data-type=taskList]>li>div]:min-w-0 [&_ul[data-type=taskList]>li>div]:flex-1 [&_ul[data-type=taskList]>li>div>p]:my-0",
  "[&_ul[data-type=taskList]>li>label>input]:size-3.5 [&_ul[data-type=taskList]>li>label>input]:accent-primary",
  "[&_ul[data-type=taskList]>li[data-checked=true]>div]:text-muted-foreground [&_ul[data-type=taskList]>li[data-checked=true]>div]:line-through",
  "[&_blockquote]:my-2 [&_blockquote]:border-l-2 [&_blockquote]:border-border [&_blockquote]:pl-3 [&_blockquote]:text-muted-foreground",
  "[&_code]:rounded [&_code]:bg-muted [&_code]:px-1 [&_code]:py-0.5 [&_code]:text-[0.85em]",
  "[&_pre]:my-2 [&_pre]:overflow-x-auto [&_pre]:rounded-xl [&_pre]:bg-muted [&_pre]:p-3",
  "[&_pre_code]:bg-transparent [&_pre_code]:p-0",
  "[&_hr]:my-4 [&_hr]:border-border",
  "[&_a]:text-primary [&_a]:underline [&_a]:underline-offset-4",
  "[&_strong]:font-semibold",
].join(" ")

const PLACEHOLDER_CLASS =
  "[&_p.is-editor-empty:first-child]:before:pointer-events-none [&_p.is-editor-empty:first-child]:before:float-left [&_p.is-editor-empty:first-child]:before:h-0 [&_p.is-editor-empty:first-child]:before:text-muted-foreground [&_p.is-editor-empty:first-child]:before:content-[attr(data-placeholder)]"

function readMarkdown(editor: Editor) {
  return (
    editor.storage as unknown as { markdown: MarkdownStorage }
  ).markdown.getMarkdown()
}

// tiptap-markdown only marks bullet and ordered lists as tight, so a task list
// would serialize with a blank line between every item. The attribute is not
// rendered — it exists purely for the markdown serializer.
const TightTaskList = TaskList.extend({
  addAttributes() {
    return {
      ...this.parent?.(),
      tight: { default: true, rendered: false },
    }
  },
})

function buildExtensions(placeholder?: string) {
  return [
    StarterKit.configure({
      heading: { levels: [1, 2, 3] },
      // Markdown has no underline, so keeping it would let users write
      // formatting that silently disappears on save.
      underline: false,
    }),
    TightTaskList,
    // Nesting lets a checklist item hold sub-items, which is what `- [ ]`
    // indented under another item means in markdown.
    TaskItem.configure({ nested: true }),
    Placeholder.configure({ placeholder: placeholder ?? "" }),
    Markdown.configure({
      html: false,
      transformPastedText: true,
      transformCopiedText: true,
    }),
  ]
}

/**
 * Markdown-backed rich text editor. `value` is markdown in and markdown out —
 * the ProseMirror document is only an implementation detail.
 */
function MarkdownEditor({
  value,
  onChange,
  disabled,
  placeholder,
  className,
}: {
  value: string
  onChange: (markdown: string) => void
  disabled?: boolean
  placeholder?: string
  className?: string
}) {
  const editor = useEditor({
    extensions: buildExtensions(placeholder),
    content: value,
    editable: !disabled,
    immediatelyRender: false,
    // The toolbar mirrors the caret's formatting, so it has to follow selection
    // changes and not only document changes.
    shouldRerenderOnTransaction: true,
    editorProps: {
      attributes: {
        class: cn(CONTENT_CLASS, PLACEHOLDER_CLASS, "min-h-40 px-3 py-2.5"),
      },
    },
    onUpdate: ({ editor }) => onChange(readMarkdown(editor)),
  })

  React.useEffect(() => {
    editor?.setEditable(!disabled)
  }, [editor, disabled])

  return (
    <div
      className={cn(
        "flex flex-col overflow-hidden rounded-2xl border border-input bg-input/30 focus-within:border-ring focus-within:ring-[3px] focus-within:ring-ring/50",
        disabled && "pointer-events-none opacity-50",
        className
      )}
    >
      <Toolbar editor={editor} />
      <EditorContent editor={editor} className="max-h-72 overflow-y-auto" />
    </div>
  )
}

/** Read-only render of a markdown string, styled like the editor. */
function MarkdownView({
  value,
  className,
}: {
  value: string
  className?: string
}) {
  const editor = useEditor(
    {
      extensions: buildExtensions(),
      content: value,
      editable: false,
      immediatelyRender: false,
      editorProps: {
        attributes: {
          // Checkboxes stay rendered but inert here: toggling one would only
          // revert on the next transaction since nothing writes it back.
          class: cn(
            CONTENT_CLASS,
            "[&_ul[data-type=taskList]>li>label]:pointer-events-none"
          ),
        },
      },
    },
    [value]
  )

  return <EditorContent editor={editor} className={className} />
}

type ToolbarAction = {
  labelKey: TranslationKey
  icon: IconSvgElement
  isActive: (editor: Editor) => boolean
  run: (editor: Editor) => void
}

const ACTIONS: ToolbarAction[] = [
  {
    labelKey: "editor.bold",
    icon: TextBoldIcon,
    isActive: (editor) => editor.isActive("bold"),
    run: (editor) => editor.chain().focus().toggleBold().run(),
  },
  {
    labelKey: "editor.italic",
    icon: TextItalicIcon,
    isActive: (editor) => editor.isActive("italic"),
    run: (editor) => editor.chain().focus().toggleItalic().run(),
  },
  {
    labelKey: "editor.strike",
    icon: TextStrikethroughIcon,
    isActive: (editor) => editor.isActive("strike"),
    run: (editor) => editor.chain().focus().toggleStrike().run(),
  },
  {
    labelKey: "editor.heading2",
    icon: Heading02Icon,
    isActive: (editor) => editor.isActive("heading", { level: 2 }),
    run: (editor) => editor.chain().focus().toggleHeading({ level: 2 }).run(),
  },
  {
    labelKey: "editor.heading3",
    icon: Heading03Icon,
    isActive: (editor) => editor.isActive("heading", { level: 3 }),
    run: (editor) => editor.chain().focus().toggleHeading({ level: 3 }).run(),
  },
  {
    labelKey: "editor.bulletList",
    icon: LeftToRightListBulletIcon,
    isActive: (editor) => editor.isActive("bulletList"),
    run: (editor) => editor.chain().focus().toggleBulletList().run(),
  },
  {
    labelKey: "editor.orderedList",
    icon: LeftToRightListNumberIcon,
    isActive: (editor) => editor.isActive("orderedList"),
    run: (editor) => editor.chain().focus().toggleOrderedList().run(),
  },
  {
    labelKey: "editor.taskList",
    icon: CheckListIcon,
    isActive: (editor) => editor.isActive("taskList"),
    run: (editor) => editor.chain().focus().toggleTaskList().run(),
  },
  {
    labelKey: "editor.quote",
    icon: LeftToRightBlockQuoteIcon,
    isActive: (editor) => editor.isActive("blockquote"),
    run: (editor) => editor.chain().focus().toggleBlockquote().run(),
  },
  {
    labelKey: "editor.code",
    icon: SourceCodeIcon,
    isActive: (editor) => editor.isActive("code"),
    run: (editor) => editor.chain().focus().toggleCode().run(),
  },
]

function Toolbar({ editor }: { editor: Editor | null }) {
  const { t } = useI18n()

  if (!editor) {
    return null
  }

  return (
    <div className="flex flex-wrap items-center gap-0.5 border-b border-border/70 bg-muted/40 p-1">
      {ACTIONS.map((action) => (
        <Button
          key={action.labelKey}
          type="button"
          variant="ghost"
          size="icon-sm"
          aria-label={t(action.labelKey)}
          aria-pressed={action.isActive(editor)}
          className={cn(
            "text-muted-foreground",
            action.isActive(editor) && "bg-muted text-foreground"
          )}
          onClick={() => action.run(editor)}
        >
          <HugeiconsIcon icon={action.icon} />
        </Button>
      ))}
    </div>
  )
}

export { MarkdownEditor, MarkdownView }
