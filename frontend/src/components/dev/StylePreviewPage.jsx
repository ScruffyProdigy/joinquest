import { Button } from '../ui/button'

export default function StylePreviewPage() {
  return (
    <main className="min-h-screen bg-background text-foreground p-10 flex flex-col gap-8">
      <div>
        <h1 className="font-heading text-4xl font-bold">Tailwind + shadcn/ui style check</h1>
        <p className="font-sans text-muted-foreground mt-2">
          Confirms the Tailwind v4 + shadcn/ui pipeline renders the ported theme tokens correctly.
        </p>
      </div>

      <section className="flex flex-col gap-4">
        <h2 className="font-heading text-xl font-semibold">Buttons</h2>
        <div className="flex flex-wrap gap-3">
          <Button>Default</Button>
          <Button variant="secondary">Secondary</Button>
          <Button variant="outline">Outline</Button>
          <Button variant="ghost">Ghost</Button>
          <Button variant="destructive">Destructive</Button>
        </div>
      </section>

      <section className="flex flex-col gap-2">
        <h2 className="font-heading text-xl font-semibold">Fonts</h2>
        <p className="font-heading text-2xl">Bricolage Grotesque (heading)</p>
        <p className="font-sans text-lg">Plus Jakarta Sans (body)</p>
        <p className="font-mono-display text-sm">DM Mono (mono display)</p>
      </section>
    </main>
  )
}
