import * as React from "react"

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { useResetOnOpen } from "@/hooks/use-reset-on-open"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Input } from "@/components/ui/input"
import { Spinner } from "@/components/ui/spinner"

/** What has to be typed out before a delete button comes alive. */
const DELETE_CONFIRM_WORD = "confirm"

/**
 * The confirmation in front of anything that cannot be undone. It owns the
 * running and failed states only: `onConfirm` does the work, says whatever it
 * wants to say on success and closes the dialog, and throwing leaves the dialog
 * open with the error in it so the action can be tried again.
 */
function ConfirmDestructiveDialog({
  open,
  onOpenChange,
  title,
  description,
  errorMessage,
  cancelLabel,
  confirmLabel,
  busyLabel,
  confirmPlaceholder,
  onConfirm,
  forceOverlay = false,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description: string
  /** Shown in place of closing when `onConfirm` rejects. */
  errorMessage: string
  cancelLabel: string
  confirmLabel: string
  /** Replaces the confirm label while the action runs. */
  busyLabel?: string
  /**
   * Asks for the confirmation word to be typed out first, with this as the
   * input's placeholder. Left out, the button is live from the start.
   */
  confirmPlaceholder?: string
  onConfirm: () => Promise<void>
  /** For a confirmation opened from inside another dialog. */
  forceOverlay?: boolean
}) {
  const [confirmText, setConfirmText] = React.useState("")
  const [busy, setBusy] = React.useState(false)
  const [hasError, setHasError] = React.useState(false)

  const needsWord = confirmPlaceholder !== undefined
  const confirmed = !needsWord || confirmText === DELETE_CONFIRM_WORD

  // `onConfirm` closes the dialog itself, which never runs handleOpenChange —
  // so the running flag is cleared on the way back in instead.
  useResetOnOpen(open, () => setBusy(false))

  function handleOpenChange(next: boolean) {
    if (busy) {
      return
    }
    if (!next) {
      setConfirmText("")
      setHasError(false)
    }
    onOpenChange(next)
  }

  async function confirm() {
    if (!confirmed) {
      return
    }

    setBusy(true)
    setHasError(false)
    try {
      await onConfirm()
    } catch {
      setHasError(true)
      setBusy(false)
    }
  }

  return (
    <AlertDialog open={open} onOpenChange={handleOpenChange}>
      <AlertDialogContent forceOverlay={forceOverlay}>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription>{description}</AlertDialogDescription>
        </AlertDialogHeader>
        {hasError && (
          <Alert variant="destructive">
            <AlertDescription>{errorMessage}</AlertDescription>
          </Alert>
        )}
        {needsWord && (
          <Input
            autoFocus
            value={confirmText}
            onChange={(event) => setConfirmText(event.target.value)}
            placeholder={confirmPlaceholder}
            disabled={busy}
          />
        )}
        <AlertDialogFooter>
          <AlertDialogCancel disabled={busy}>{cancelLabel}</AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            disabled={!confirmed || busy}
            onClick={confirm}
          >
            {busy && <Spinner data-icon="inline-start" />}
            {busy ? (busyLabel ?? confirmLabel) : confirmLabel}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

export { ConfirmDestructiveDialog }
