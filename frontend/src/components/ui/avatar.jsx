import { cn } from '../../lib/utils'

const AVATAR_SIZE_CLASSES = {
  sm: 'size-8 text-xs',
  md: 'size-12 text-base',
  lg: 'size-16 text-xl',
}

function Avatar({ className, size = 'md', ...props }) {
  return (
    <span
      data-slot="avatar"
      className={cn(
        'relative inline-flex shrink-0 items-center justify-center overflow-hidden rounded-full bg-muted font-heading font-semibold text-foreground',
        AVATAR_SIZE_CLASSES[size],
        className,
      )}
      {...props}
    />
  )
}

function AvatarImage({ className, ...props }) {
  return <img data-slot="avatar-image" className={cn('aspect-square size-full object-cover', className)} {...props} />
}

function AvatarFallback({ className, ...props }) {
  return <span data-slot="avatar-fallback" className={cn('flex size-full items-center justify-center', className)} {...props} />
}

export { Avatar, AvatarImage, AvatarFallback }
