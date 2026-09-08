import { cn } from '../../lib/utils'
import { linkVariants } from './link-variants'

/**
 * The one link primitive. `href` picks the element, so a call site never has to
 * decide: navigation gets a real anchor the browser can open in a new tab, and
 * an action that only runs a handler gets a button the keyboard and screen
 * reader can recognise as one.
 */
function Link({ className, variant, href, type, ...props }) {
  const classes = cn(linkVariants({ variant, className }))

  if (href === undefined) {
    return <button data-slot="link" type={type ?? 'button'} className={classes} {...props} />
  }

  return <a data-slot="link" href={href} className={classes} {...props} />
}

export { Link }
