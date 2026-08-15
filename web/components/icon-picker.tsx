import * as React from "react"
import { HugeiconsIcon, type IconSvgElement } from "@hugeicons/react"

import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import { useI18n } from "@/lib/i18n/context"
import { getListIcon, LIST_ICONS } from "@/lib/list-icons"

function IconPicker({
  value,
  onChange,
  disabled,
  icons = LIST_ICONS,
  resolveIcon = getListIcon,
}: {
  value: string
  onChange: (icon: string) => void
  disabled?: boolean
  icons?: Record<string, IconSvgElement>
  resolveIcon?: (icon: string | undefined | null) => IconSvgElement
}) {
  const { t } = useI18n()
  const [open, setOpen] = React.useState(false)

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={
          <Button
            type="button"
            variant="outline"
            size="icon"
            disabled={disabled}
            aria-label={t("list.iconLabel")}
            className="shrink-0"
          />
        }
      >
        <HugeiconsIcon icon={resolveIcon(value)} />
      </PopoverTrigger>
      <PopoverContent align="start" className="w-64">
        <div className="grid max-h-64 grid-cols-6 gap-1.5 overflow-y-auto">
          {Object.entries(icons).map(([name, icon]) => (
            <Button
              key={name}
              type="button"
              variant={name === value ? "secondary" : "ghost"}
              size="icon-lg"
              className={cn(name === value && "ring-1 ring-primary/40")}
              onClick={() => {
                onChange(name)
                setOpen(false)
              }}
            >
              <HugeiconsIcon icon={icon} />
              <span className="sr-only">{name}</span>
            </Button>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  )
}

export { IconPicker }
