# Catalog art — re-roll prompts (draft 2)

14 replacements from the draft 1 review: **5 heroes, 9 icons.** Everything else from draft 1 is keeping.

**Two games were also renamed** (name collisions inside the set): *Closing Crew* → **Skeleton Staff** (clashed with Vent Crew), *Shared Ink* → **Handiwork** (clashed with Inkbolt). Notion is already updated; the filenames below use the new slugs.

Same rules as before: one prompt per message, and reject anything with text in it. Which chat to use is now per-game — see below.

**The icon fix**, applied to all 9: *one dominant subject at ≥60% of the frame, in strong value contrast against its background.* This works inside each band's existing style — Siege Bell is Mid-band and painterly and reads perfectly at 120px, so "detailed" never required "muddy."

**The hero fix**, applied to the 3 drifted ones: the negatives were there and got ignored, so draft 2 leads with a hard style declaration *first* and names the medium explicitly, rather than stacking prohibitions at the end.

---

## Which chat to use

Whatever ChatGPT already produced in a thread stays in context as the reference for "what this game looks like." That helps when the style was right and only the composition was wrong. It **hurts** when the style itself was the failure, because the bad image anchors it toward repeating exactly the mistake.

### 🆕 START A FRESH CHAT — 5 games

The model did something we don't want repeated, so don't let it see the old one.

| Game | Assets | Why fresh |
|---|---|---|
| Word Hunt | hero + icon | The alphabet is in context; it will drift back to A, B, C |
| Know Better | hero | Drifted to soft-shadowed 3D — that render is the anchor |
| Vent Crew | hero + icon | Drifted painterly; both assets inherit the wrong medium |
| Skeleton Staff | hero + icon | Drifted near-photoreal, same problem |
| Two Camps | icon | Your original chat has RPSLR hand-stickers in it from the misfire — contaminated regardless |

**Where both hero and icon are being redone — Word Hunt, Vent Crew, Skeleton Staff — do the pair together in ONE fresh chat, hero first.** The new icon then matches the new hero, which preserves the per-game art-direction consistency the card-versus-table comparison depends on.

### ♻️ REUSE THE GAME'S ORIGINAL CHAT — 6 assets

Style was right; only the composition needs changing. Keeping context preserves the palette and rendering that already matched the hero.

Kettle & Crown icon · Sunderline icon · Counterpoise icon · Crumbfall icon · Both Hands icon · Handiwork hero

**Prefix the prompt with this line**, or an in-thread regeneration will nudge the old composition rather than rebuild it — which for Kettle & Crown means keeping the scattered coins that are the whole reason it fails at 120px:

> New version, replacing the previous image. Ignore the previous composition entirely and follow this exactly:

---

# Word Hunt — re-roll both  🆕 FRESH CHAT (with its icon, hero first)

The alphabet is the problem. A–Z in order reads as a children's spelling toy.

`word-hunt-hero.png`

```
Bold geometric letterpress-style poster illustration, completely flat. No gradients, no shading, no rendered lighting, no soft shadows, no depth — pure flat colour shapes, like screen-printed paper. A complete 6 by 4 grid of chunky square tiles seen from directly above, filling the left two-thirds of a wide frame with no empty cells, each tile bearing one large clearly-formed capital letter. The letters are a random jumble — deliberately NOT in alphabetical order, spelling nothing, with several letters repeating, like a word-search board. Five loose tiles scattered at angles across the open space at the right. Amber, cream, deep navy, one single teal tile. Crisp hard edges, high contrast, graphic and confident. Letters on the tiles only — no other text, no words, no titles, no numbers, no logos. Horizontal 4:3 landscape.
```

`word-hunt-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop with the subject clear of the corners. Bold geometric letterpress style, completely flat — no gradients, no shading, no soft shadows, no depth. Three chunky square letter tiles overlapping at slight angles and filling at least 60% of the frame, in strong contrast against a deep navy ground. The three letters are R, K and W — deliberately not A, B, C, not alphabetical, spelling nothing. Amber and cream tiles, deep navy background, crisp hard edges. Letters on the tiles only — no other text, no words, no numbers, no logos. Square 1:1.
```

# Know Better — re-roll hero  🆕 FRESH CHAT

Draft 1 picked up soft drop shadows and a gradient backdrop, drifting a band heavier than a barrier-1 party game should look.

`know-better-hero.png`

```
Flat vector illustration in the style of a screen-printed poster. Completely flat colour — no gradients, no soft shadows, no studio lighting, no reflections, no 3D rendering of any kind. A row of four large rounded buzzer buttons drawn as simple flat circles and rounded rectangles across a plain single-colour backdrop, the second one pressed down with three short straight impact lines above it. Warm yellow, off-white, coral, one deep plum. Thick uniform outlines, bold and playful, deliberately simple. No text, no words, no letters, no numbers, no logos. Horizontal 4:3 landscape.
```

# Two Camps — re-roll icon  🆕 FRESH CHAT (original is contaminated by the RPSLR misfire)

At 120px the two groups read as abstract vertical stripes.

`two-camps-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Mid-century screenprint, paper grain, slight ink misregistration, flat colour blocking. Two large simplified human silhouettes in profile facing each other across a narrow central gap, each occupying a full half of the frame, heads near the top edge — bold burnt orange on the left, deep teal on the right, against a cream ground. Only two figures, large and unmistakable, not a crowd. Strong value contrast. No text, no letters, no numbers, no logos. Square 1:1.
```

# Vent Crew — re-roll both  🆕 FRESH CHAT (with its icon, hero first)

Draft 1 rendered as painterly concept art rather than comic-book ink, drifting heavier than a barrier-2 game.

`vent-crew-hero.png`

```
Flat comic-book ink illustration in the style of a printed graphic novel page. Bold black ink linework, flat spot colour, visible halftone dot texture. No painterly rendering, no volumetric light, no atmospheric haze, no photorealism, no soft gradients. A cramped ship corridor running left to right across a wide frame, exposed pipes along the walls and a floor vent in the foreground, one figure walking away at the far end, a second figure half-hidden behind a bulkhead in the near foreground watching them. Cool blue and steel grey flat fills, one warm amber light source, heavy black ink shadows. Graphic and printed, not painted. No text, no words, no letters, no numbers, no logos. Horizontal 4:3 landscape.
```

`vent-crew-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Flat comic-book ink with bold black linework, flat spot colour and halftone dots — not painterly, no soft gradients. One large floor vent grille seen head-on, occupying at least 60% of the frame, bright warm amber light blazing through its slats against a deep flat blue ground. Strong value contrast, high graphic clarity. No text, no letters, no numbers, no logos. Square 1:1.
```

# Handiwork — re-roll hero  ♻️ REUSE this game's original chat — the one you generated *Shared Ink* in, before the rename

Draft 1 put brushes around a completely blank centre, which reads as unfinished rather than as a shared canvas.

`handiwork-hero.png`

```
Risograph print style, two or three flat spot colours with visible grain and slight misregistration. No gradients, no rendered shading, no soft shadows, no depth. Many simplified hands reaching in from the left and right edges of a wide frame, each holding a different brush, all working on a single large canvas at the centre that is half-covered in bold overlapping brushstrokes and colour blocks — clearly a painting in progress, not an empty rectangle. Teal, coral, cream. Warm, communal, slightly imperfect print texture. No text, no words, no letters, no numbers, no logos. Horizontal 4:3 landscape.
```

# Skeleton Staff — re-roll both  🆕 FRESH CHAT (with its icon, hero first)

Hero drifted to near-photoreal; icon is a dark rectangle at 120px.

`skeleton-staff-hero.png`

```
Stylised cel-shaded illustration with flat colour areas and hard-edged shadows, in the style of a graphic game poster. No photorealism, no volumetric fog, no fine surface texture, no cinematic depth of field. A retail stockroom after hours, long shelving units running left to right, one overhead striplight casting a hard flat pool of light, a mop and bucket abandoned mid-aisle. Rust orange, deep charcoal, sickly green. Tense and still, but graphic and readable rather than photographic. No text, no words, no letters, no numbers, no signage, no logos. Horizontal 4:3 landscape.
```

`skeleton-staff-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Stylised cel-shaded illustration, flat colour areas, hard-edged shadows, no photorealism. One large overhead striplight fixture seen straight on, occupying at least 60% of the frame, blazing bright sickly green-white against a deep charcoal ground, with a hard rust-orange light spill below it. Strong value contrast, one unmistakable dominant shape. No text, no letters, no numbers, no signage, no logos. Square 1:1.
```

# Kettle & Crown — re-roll icon  ♻️ REUSE the Kettle & Crown chat

At 120px the crest is dense gold-on-brown and unidentifiable.

`kettle-and-crown-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Densely detailed painterly illustration with fine linework and aged parchment texture, oil painting quality. One large ornate heraldic shield with a crown above it, filling at least 70% of the frame, rendered in bright antique gold and pale cream against a very dark near-black brown ground. The shield must read as a single strong silhouette at small size — keep surrounding clutter minimal, no coins, no scattered objects. Rich and heavy but with unmistakable shape and strong value contrast. No text, no letters, no numbers, no logos. Square 1:1.
```

# Sunderline — re-roll icon  ♻️ REUSE the Sunderline chat

Dark purple on dark purple; the shield barely holds.

`sunderline-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Cinematic concept art, dense detail, painterly rendering, dramatic rim lighting. One large armoured shield filling at least 70% of the frame, split down the centre by a blazing bright violet fracture that is the brightest element in the image, against a very dark near-black ground. The shield silhouette must be unmistakable at small size — strong value contrast between the pale rim-lit shield edge, the brilliant violet crack, and the black background. No busy background detail. No text, no letters, no numbers, no logos. Square 1:1.
```

# Counterpoise — re-roll icon  ♻️ REUSE the Counterpoise chat

Grey on grey with thin linework; no contrast at all when small.

`counterpoise-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Fine engraved etching style with dense crosshatching and technical illustration precision. One large brass balance scale filling at least 70% of the frame, tipped hard to one side, rendered in near-black ink with heavy confident linework against a bright pale cream ground — maximum value contrast, dark subject on light background. One pan visibly clockwork, the other visibly root-like. Thick enough linework to hold at small size; no fine grey tones, no washed-out mid-greys. No text, no letters, no numbers, no logos. Square 1:1.
```

# Crumbfall — re-roll icon  ♻️ REUSE the Crumbfall chat

Near-black, and the lime creature is a single dot. Inverting the composition keeps the deliberate mis-signal while making it legible.

`crumbfall-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Densely detailed painterly digital illustration, dramatic volumetric lighting, fine texture, intricate rendering. One enormous dark predatory creature's head and jaws curling down from the top of the frame and filling at least 60% of it, rim-lit in acid lime green so its silhouette reads clearly, with one small bright lime orb glowing far below it for scale. Deep forest green and near-black, with the lime rim light as the brightest element. Serious, weighty and grand — an epic-strategy look. Strong value contrast; the creature must be unmistakable at small size. No text, no letters, no numbers, no logos. Square 1:1.
```

# Both Hands — re-roll icon  ♻️ REUSE the Both Hands chat

Mechanical detail turns to noise at 120px.

`both-hands-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Clean isometric technical illustration, flat colour, precise linework, no photorealism. One large mechanical padlock split cleanly into two halves pulling apart with a bright gap between them, filling at least 60% of the frame, in bright teal and warm cream against a dark charcoal ground. Simplify the internals to a few large gears and a single bold keyhole in each half — no dense mechanism detail, no small parts. Strong value contrast, unmistakable silhouette at small size. No text, no letters, no numbers, no measurement markings, no logos. Square 1:1.
```
