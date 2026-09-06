# Catalog fixture — art prompts

40 prompts for the 20 demo-fixture games: one 4:3 landscape hero and one 1:1 icon each. Written for pasting into ChatGPT.

Companion to 🎲 Catalog Games (demo fixture) and the coverage matrix page in Notion.

---

## Before you start

**Start a fresh chat every 2–3 games.** ChatGPT carries context between images in a conversation and drifts toward making them a matching set. That is the opposite of what this fixture needs — the premise is twenty games from twenty different in-house teams, and accidental house-style consistency would flatten the thing we're testing. Definitely start fresh between bands.

**Paste one prompt per message.** Batching makes it blend subjects.

**Two rules are load-bearing:**

1. Every prompt ends with **"no text, no words, no letters, no numbers, no logos."** ChatGPT will otherwise add signage, racing numbers, and gibberish lettering — and a title lockup sits over the hero image. Text in the art is a defect no matter how good it looks.
2. The **negative style constraints on the Cartoon band** ("no gradients, no shading, no texture, no soft shadows, no depth"). Image models drift painterly by default. That drift is exactly the failure this fixture exists to catch: a ten-minute party game that mis-signals as heavy before anyone reads a word. A Cartoon-band image with soft shading has failed the brief even if it's attractive.

**Regenerate if:** any text appears · a Cartoon-band image has gradients or rendered lighting · the composition came out square or portrait (heroes are landscape) · an icon reads as a flat pictogram rather than full-bleed artwork · an icon's subject runs into the corners (it gets rounded off).

**Start with #8 Whisper Sketch.** It's the hardest constraint case in the set — four flat colours, thick outlines, a deliberately badly-drawn cat, no shading anywhere. If ChatGPT handles that one cleanly it will handle the other 39.

---

## Why two assets per game

These are **two competing card systems**, not a hero plus a supporting mark.

- **4:3 landscape hero** — a poster-style card grid. Horizontal media is the direct lever on how many games fit on a mobile screen at once, which is the discovery problem. The Figma demo currently shows a roughly 1:1 card, one per screen, in a 510px column at every width.
- **1:1 icon** — an Apple Arcade / Steam-style table view, where the square icon anchors each row. At 88–120px this is the *primary* visual, not a 48px mark, so the icons are briefed as full-bleed app-icon artwork rather than flat pictograms.

For the comparison to test **layout**, both assets for a given game must share art direction — same style vocabulary, same palette, same band. If the icons were flat glyphs and the heroes were rich illustrations, the study would be measuring art quality and drawing conclusions about card systems.

The icon is **not a crop of the hero.** Square is composed as square.

## Size spec for developers

| Asset | Dimensions | Notes |
|---|---|---|
| Hero | **1200 × 900** (4:3) | PNG or WebP, sRGB, no transparency, no baked-in title |
| Icon | **1024 × 1024** (1:1) | Full bleed, keep the subject clear of the corners — it gets rounded |

Both are 2x for their largest expected render, so they downscale cleanly and there's headroom if the card grows.

One gap worth deciding on: direct-linking means the Discord/social unfurl *is* the card for a cold visitor, and unfurls want **1200 × 630** (1.91:1). A 4:3 hero cannot crop to that without losing a third of the composition. Either that's a third required asset, or the platform composes it from the hero plus the accent colour and title — worth settling before this becomes a dev-facing spec.

---

## The three bands

Art band tracks Barrier — simpler games get more cartoonish treatment, heavier games get more detail. The band sets the *weight*, not the look: each game has its own style vocabulary on purpose.

| Band | Count | Weight |
|---|---|---|
| Cartoon | 9 | Flat, bold, high-chroma, zero rendering |
| Mid | 7 | Stylised, some texture, controlled shading |
| Detailed | 4 | Painterly, dense, muted, heavy |

**Two are deliberately mismatched — do not correct them:**

- **#7 Salt the Well** — Barrier 3 given Cartoon art. Does cute art make a heavy game look light?
- **#18 Crumbfall** — Barrier 1 given Detailed art. Does over-detailed art make a trivial game look heavy? This is the direction models drift on their own, so it's the more common real-world failure.

---

# 1 · Word Hunt — Cartoon

`word-hunt-hero.png`

```
Bold geometric letterpress-style poster illustration, completely flat. No gradients, no shading, no rendered lighting, no soft shadows, no depth — pure flat colour shapes, like screen-printed paper. A grid of chunky square tiles seen from directly above filling the left two-thirds of a wide frame, each tile bearing one large clearly-formed capital letter, several tiles lifted and scattered across the open space at the right. Amber, cream, deep navy, one single teal tile. Crisp hard edges, high contrast, graphic and confident. Letters on the tiles only — no other text, no words, no titles, no numbers, no logos. Horizontal 4:3 landscape.
```

`word-hunt-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop with the subject clear of the corners. Bold geometric letterpress style, completely flat — no gradients, no shading, no soft shadows, no depth. Three chunky square letter tiles overlapping at slight angles, filling the frame, each bearing one large clearly-formed capital letter. Amber, cream, deep navy. Crisp hard edges, high contrast. Rich enough to hold at 240px, readable at 88px. Letters on the tiles only — no other text, no words, no numbers, no logos. Square 1:1.
```

# 2 · Know Better — Cartoon

`know-better-hero.png`

```
Chunky stylised 3D toy render, matte plastic surfaces, soft even studio light, no dramatic shadows, no realistic texture, no photorealism. A row of oversized rounded buzzer buttons on stubby stands spread across a plain flat backdrop, one pressed down mid-bounce. Warm yellow, off-white, coral, one deep plum. Playful, tactile, uncluttered. No text, no words, no letters, no numbers, no logos. Horizontal 4:3 landscape.
```

`know-better-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Chunky stylised 3D toy render, matte plastic, soft even studio light, no dramatic shadows, no photorealism. One oversized rounded buzzer button filling most of the frame, caught mid-press, two smaller ones behind it. Warm yellow, off-white, coral, one deep plum. Playful and tactile. No text, no letters, no numbers, no logos. Square 1:1.
```

# 3 · Two Camps — Mid

`two-camps-hero.png`

```
Mid-century screenprint poster, limited palette, visible paper grain, slight ink misregistration. Two stylised groups of simplified figures facing each other from the left and right edges of a wide frame across a narrow gap at the centre, one figure in each group standing forward from the rest. Burnt orange, cream, teal, black. Flat colour blocking with light texture only. Confident retro graphic design. No text, no words, no letters, no numbers, no logos. Horizontal 4:3 landscape.
```

`two-camps-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Mid-century screenprint, paper grain, slight ink misregistration, flat colour blocking. Two stylised groups of simplified figures facing each other across a narrow central gap, filling the frame edge to edge. Burnt orange, cream, teal, black. Confident retro graphic design. No text, no letters, no numbers, no logos. Square 1:1.
```

# 4 · Rock Paper Scissors Lizard Robot — Cartoon

`rpslr-hero.png`

```
Thick-outline sticker-art illustration, heavy black keylines of even weight, flat fills. No gradients, no shading, no texture, no soft shadows, no depth — like vinyl stickers laid on flat paper. Five cartoon hand shapes arranged in a wide ring across the frame, each pointing at two others, every shape with a white sticker border. Mint green, hot coral, cream, black. Bouncy, bold, immediately legible. No text, no words, no letters, no numbers, no logos. Horizontal 4:3 landscape.
```

`rpslr-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Thick-outline sticker art, heavy black keylines, flat fills, no gradients, no shading, no soft shadows, no depth. Three cartoon hand shapes overlapping and filling the frame, each with a white sticker border. Mint green, hot coral, cream, black. Bouncy and bold. No text, no letters, no numbers, no logos. Square 1:1.
```

# 5 · Vent Crew — Mid

`vent-crew-hero.png`

```
Comic-book ink illustration with flat colour and visible halftone dots. A cramped ship corridor running left to right across a wide frame, exposed pipes and a floor vent, one figure walking away at the far end and a second half-hidden behind a bulkhead in the near foreground watching them. Cool blue, steel grey, one warm amber light source. Heavy black ink shadows, controlled texture, graphic novel style. No text, no words, no letters, no numbers, no logos. Horizontal 4:3 landscape.
```

`vent-crew-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Comic-book ink with flat colour and visible halftone dots, heavy black shadows. A floor vent grille seen head-on filling the frame, one amber glow leaking through the slats, a silhouette barely visible behind it. Cool blue, steel grey, warm amber. Graphic novel style. No text, no letters, no numbers, no logos. Square 1:1.
```

# 6 · Tell Nothing — Mid

`tell-nothing-hero.png`

```
Gouache painting, visible brush texture, matte opaque colour, soft edges but no photorealism. A wide grid of blank cards laid flat across a table, one card turned face down and separated slightly from the others, a hand withdrawing from it at the right edge. Violet, warm grey, chalk white, one deep crimson card. Quiet, considered, slightly tense. No text, no words, no letters, no numbers, no writing on the cards, no logos. Horizontal 4:3 landscape.
```

`tell-nothing-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Gouache painting, visible brush texture, matte opaque colour. Four blank cards fanned out and filling the frame, one of them deep crimson and turned away from the rest. Violet, warm grey, chalk white, crimson. Quiet and slightly tense. No text, no letters, no numbers, no writing on the cards, no logos. Square 1:1.
```

# 7 · Salt the Well — Cartoon ⚠ DELIBERATE MIS-SIGNAL

Barrier 3 game given Cartoon art on purpose. Do not make this look serious or heavy — the point is that it looks lighter than it plays.

`salt-the-well-hero.png`

```
Cut-paper storybook illustration, flat layered paper shapes with clean edges. No gradients, no rendered shading, no soft shadows, no depth, no realistic texture. A cheerful little village well in a wide snowy meadow, small rounded cottages spread along the horizon behind it, simplified figures carrying buckets. Sage green, cream, sky blue, warm ochre. Charming, gentle, picture-book friendly. No text, no words, no letters, no numbers, no logos. Horizontal 4:3 landscape.
```

`salt-the-well-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Cut-paper storybook illustration, flat layered paper shapes, clean edges, no gradients, no rendered shading, no depth. A round stone well with a small pitched roof at the centre, two rounded cottages and snow behind it, filling the frame. Sage green, cream, sky blue, warm ochre. Charming and picture-book friendly. No text, no letters, no numbers, no logos. Square 1:1.
```

# 8 · Whisper Sketch — Cartoon ★ START HERE

`whisper-sketch-hero.png`

```
Flat vector poster illustration, thick uniform black outlines, four flat colours only. Absolutely no gradients, no shading, no texture, no rendered lighting, no soft shadows, no depth — like a screen print on paper. A chain of four crude childlike doodles of a cat running left to right across a wide frame, connected by bold arrows, each drawing more distorted and wrong than the one before, on lined notepaper. Hot pink, cream, black, one teal accent. Playful, graphic, deliberately simple. No text, no words, no letters, no numbers, no logos. Horizontal 4:3 landscape.
```

`whisper-sketch-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Flat vector, thick uniform black outlines, four flat colours only, absolutely no gradients, no shading, no texture, no soft shadows, no depth. One large crude wobbly childlike doodle of a cat, deliberately badly drawn, filling the frame on lined notepaper. Hot pink, cream, black, one teal accent. Playful and deliberately simple. No text, no letters, no numbers, no logos. Square 1:1.
```

# 9 · Shared Ink — Cartoon

`shared-ink-hero.png`

```
Risograph print style, two or three flat spot colours with visible grain and slight misregistration. No gradients, no rendered shading, no soft shadows, no depth. Many simplified hands reaching in from the left and right edges of a wide frame onto a single large blank canvas at the centre, each holding a different brush. Teal, coral, cream. Warm, communal, slightly imperfect print texture. No text, no words, no letters, no numbers, no logos. Horizontal 4:3 landscape.
```

`shared-ink-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Risograph print style, flat spot colours, visible grain, slight misregistration, no gradients, no depth. Five simplified hands reaching in from all four edges toward a blank shape at the centre, each holding a different brush. Teal, coral, cream. Warm and communal. No text, no letters, no numbers, no logos. Square 1:1.
```

# 10 · Inkbolt — Cartoon

`inkbolt-hero.png`

```
Loose marker-pen sketch style, bold felt-tip strokes, flat highlighter fills, visible speed and energy. No gradients, no rendered shading, no soft shadows, no depth, no texture beyond the marker itself. A wide whiteboard carrying a half-finished scribbled drawing, motion lines flying off to the right, a stopwatch shape in the corner. Orange-red, cream, black, one electric blue. Fast, loud, unpolished on purpose. No text, no words, no letters, no numbers, no logos. Horizontal 4:3 landscape.
```

`inkbolt-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Loose marker-pen sketch, bold felt-tip strokes, flat highlighter fills, no gradients, no rendered shading, no depth. One thick felt-tip stroke curling into a spiral across the whole frame with motion lines flying off it. Orange-red, cream, black, one electric blue. Fast, loud, unpolished. No text, no letters, no numbers, no logos. Square 1:1.
```

# 11 · Closing Crew — Mid

`closing-crew-hero.png`

```
Stylised 3D render with cel-shaded surfaces and controlled grain, moody but not photorealistic. A dim retail stockroom after hours, long shelving units running left to right into darkness, one overhead striplight still on, a mop and bucket abandoned mid-aisle. Rust orange, deep charcoal, sickly green emergency light. Tense, still, something just out of frame. No text, no words, no letters, no numbers, no signage, no logos. Horizontal 4:3 landscape.
```

`closing-crew-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Stylised 3D render, cel-shaded surfaces, controlled grain, moody but not photorealistic. One overhead striplight glowing in a dark stockroom aisle, shelving receding into shadow either side, filling the frame. Rust orange, deep charcoal, sickly green. Tense and still. No text, no letters, no numbers, no signage, no logos. Square 1:1.
```

# 12 · Kettle & Crown — Detailed

`kettle-and-crown-hero.png`

```
Densely detailed painterly board game cover illustration, fine linework, aged parchment texture, oil painting quality. A heavy wooden trading table seen across its full width, laden with ledgers, stacked coins, wax-sealed letters and a guttering candle, five ornate heraldic house crests ranged along the wall behind it. Muted olive, deep brown, antique gold. Ornate, heavy, intricate, the look of a long serious evening game. No text, no words, no letters, no numbers, no writing on the ledgers, no logos. Horizontal 4:3 landscape.
```

`kettle-and-crown-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Densely detailed painterly illustration, fine linework, aged parchment texture, oil painting quality. One ornate heraldic crest with a small crown above it, stacked coins and a wax seal at its base, filling the frame. Muted olive, deep brown, antique gold. Ornate, heavy, intricate. No text, no letters, no numbers, no logos. Square 1:1.
```

# 13 · Sunderline — Detailed

`sunderline-hero.png`

```
Cinematic concept art, dense detail, dramatic rim lighting, atmospheric haze, painterly rendering. A long contested corridor of broken fortifications stretching across the full width of the frame, armoured silhouettes advancing from both the left and right edges, violet energy fracturing the ground along the centre line. Violet, cold steel, ember orange. Epic scale, serious, heavy. No text, no words, no letters, no numbers, no logos. Horizontal 4:3 landscape.
```

`sunderline-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Cinematic concept art, dense detail, dramatic rim lighting, atmospheric haze, painterly rendering. A dark armoured shield splitting apart down the centre with violet energy fracturing through the gap, filling the frame. Violet, cold steel, ember orange. Epic and heavy. No text, no letters, no numbers, no logos. Square 1:1.
```

# 14 · Counterpoise — Detailed

`counterpoise-hero.png`

```
Fine engraved etching style, dense crosshatching, technical illustration precision, muted and cold. An old brass balance scale with a wide beam seen straight on, spanning the frame, each pan holding a completely different mechanism — clockwork on the left, organic and root-like on the right — neither pan level. Slate grey, ink black, aged paper, one thin copper accent. Precise, austere, intricate. No text, no words, no letters, no numbers, no logos. Horizontal 4:3 landscape.
```

`counterpoise-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Fine engraved etching, dense crosshatching, technical illustration precision, muted and cold. A brass balance scale tipped hard to one side, one pan clockwork and one pan root-like, filling the frame. Slate grey, ink black, aged paper, thin copper accent. Precise and austere. No text, no letters, no numbers, no logos. Square 1:1.
```

# 15 · Shale Run — Cartoon

`shale-run-hero.png`

```
Chunky low-poly 3D toy render, matte flat-shaded surfaces, no realistic texture, no dramatic lighting, no photorealism. Four stubby toy-like rally cars strung out across a wide loose dirt track mid-corner, kicking up simplified geometric dust plumes, seen from a low front three-quarter angle. Bright yellow, dust ochre, sky blue, black. Fun, punchy, immediately readable. No text, no words, no letters, no numbers, no racing numbers on the cars, no sponsor logos. Horizontal 4:3 landscape.
```

`shale-run-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Chunky low-poly 3D toy render, matte flat-shaded surfaces, no realistic texture, no dramatic lighting, no photorealism. One stubby toy rally car head-on and slightly angled, filling the frame, geometric dust plumes kicking up behind it. Bright yellow, dust ochre, sky blue, black. Fun and punchy. No text, no letters, no numbers, no racing numbers, no sponsor logos. Square 1:1.
```

# 16 · Standing Eight — Mid

`standing-eight-hero.png`

```
Bold comic ink illustration with heavy black brushwork, flat colour and halftone shading. Two stylised boxers seen from the side across a wide low-lit ring, one mid-guard at the left and one mid-swing at the right, ropes cutting horizontally across the frame, harsh overhead light. Red, cream, black, deep blue shadow. Graphic, punchy, high contrast, controlled texture. No text, no words, no letters, no numbers, no logos on shorts or ring. Horizontal 4:3 landscape.
```

`standing-eight-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Bold comic ink, heavy black brushwork, flat colour, halftone shading. One boxing glove mid-punch coming toward the viewer, filling the frame, ring ropes cutting behind it. Red, cream, black, deep blue shadow. Graphic, punchy, high contrast. No text, no letters, no numbers, no logos. Square 1:1.
```

# 17 · Siege Bell — Mid

`siege-bell-hero.png`

```
Semi-stylised video game key art, moderate detail, clean cel shading with limited texture, not photorealistic and not painterly. A large bronze bell hanging over a stone platform at the centre of a wide frame, two teams of silhouetted armoured figures converging on it from the left and right edges, dust and shafts of light in the air. Sky blue and warm bronze, cool blue shadows. Bold readable shapes, confident composition. No text, no words, no letters, no numbers, no logos. Horizontal 4:3 landscape.
```

`siege-bell-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Semi-stylised game key art, clean cel shading, limited texture, not photorealistic and not painterly. One large bronze bell filling the frame, silhouetted armoured figures small at its base, dust and a shaft of light behind. Sky blue, warm bronze, cool blue shadows. Bold readable shapes. No text, no letters, no numbers, no logos. Square 1:1.
```

# 18 · Crumbfall — Detailed ⚠ DELIBERATE MIS-SIGNAL

Barrier 1, four-minute game given Detailed art on purpose. Do not simplify this. Over-detailed art on a trivial game is the drift models produce naturally, which makes it the common real-world failure — we want a specimen.

`crumbfall-hero.png`

```
Densely detailed painterly digital illustration, dramatic volumetric lighting, fine texture, epic cinematic composition. Tiny luminous round creatures scattered across the floor of a vast dark bowl-shaped arena spanning the full width of the frame, one enormous predatory shape looming out of shadow at the right, god rays raking down, intricate rendering, ominous sense of scale. Deep forest green and near-black with acid lime highlights. Serious, weighty and grand, the look of an epic strategy game. No text, no words, no letters, no numbers, no logos. Horizontal 4:3 landscape.
```

`crumbfall-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Densely detailed painterly digital illustration, dramatic volumetric lighting, fine texture, intricate rendering. One small glowing lime-green creature dwarfed by an enormous dark predatory shape curling over it, god rays raking down, filling the frame. Deep forest green and near-black with acid lime highlights. Serious, weighty and grand. No text, no letters, no numbers, no logos. Square 1:1.
```

# 19 · Both Hands — Mid

`both-hands-hero.png`

```
Clean isometric technical illustration, flat colour with light controlled shading, precise linework, no photorealism. A complex mechanical lock cut cleanly in half down the middle, the two halves pulled apart to the left and right with a gap between them, gears and tumblers exposed on both facing surfaces. Teal, warm grey, black, one amber highlight. Orderly, diagrammatic, satisfying. No text, no words, no letters, no numbers, no measurement markings, no logos. Horizontal 4:3 landscape.
```

`both-hands-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Clean isometric technical illustration, flat colour, light controlled shading, precise linework, no photorealism. One mechanical lock split cleanly into two halves pulling apart with gears exposed on both faces, filling the frame. Teal, warm grey, black, one amber highlight. Orderly and diagrammatic. No text, no letters, no numbers, no measurement markings, no logos. Square 1:1.
```

# 20 · Cinchpoint — Cartoon

`cinchpoint-hero.png`

```
Flat geometric vector illustration with neon-bright colour, hard edges, uniform line weight. No gradients, no shading, no texture, no rendered lighting, no soft shadows, no depth. Two identical tangled loops of cord side by side across a wide dark field, the left one partly unknotted and the right one still snarled. Electric purple, hot cyan, near-black, cream. Clean, sharp, slightly hypnotic. No text, no words, no letters, no numbers, no logos. Horizontal 4:3 landscape.
```

`cinchpoint-icon.png`

```
App icon artwork, full bleed to all four edges, composed for a rounded-square crop. Flat geometric vector, neon-bright colour, hard edges, uniform line weight, no gradients, no shading, no soft shadows, no depth. One tangled loop of cord tied in a single bold knot filling the frame. Electric purple, hot cyan, near-black, cream. Clean, sharp, slightly hypnotic. No text, no letters, no numbers, no logos. Square 1:1.
```

---

## Progress

| # | Game | Band | Hero | Icon |
|---|---|---|---|---|
| 1 | Word Hunt | Cartoon | ☐ | ☐ |
| 2 | Know Better | Cartoon | ☐ | ☐ |
| 3 | Two Camps | Mid | ☐ | ☐ |
| 4 | Rock Paper Scissors Lizard Robot | Cartoon | ☐ | ☐ |
| 5 | Vent Crew | Mid | ☐ | ☐ |
| 6 | Tell Nothing | Mid | ☐ | ☐ |
| 7 | Salt the Well | Cartoon ⚠ | ☐ | ☐ |
| 8 | Whisper Sketch | Cartoon ★ | ☐ | ☐ |
| 9 | Shared Ink | Cartoon | ☐ | ☐ |
| 10 | Inkbolt | Cartoon | ☐ | ☐ |
| 11 | Closing Crew | Mid | ☐ | ☐ |
| 12 | Kettle & Crown | Detailed | ☐ | ☐ |
| 13 | Sunderline | Detailed | ☐ | ☐ |
| 14 | Counterpoise | Detailed | ☐ | ☐ |
| 15 | Shale Run | Cartoon | ☐ | ☐ |
| 16 | Standing Eight | Mid | ☐ | ☐ |
| 17 | Siege Bell | Mid | ☐ | ☐ |
| 18 | Crumbfall | Detailed ⚠ | ☐ | ☐ |
| 19 | Both Hands | Mid | ☐ | ☐ |
| 20 | Cinchpoint | Cartoon | ☐ | ☐ |

## When you're done

Images need a **stable URL** — the repo or a CDN. Notion holds only the URL, in `Hero URL (3:4)` and `Icon URL (1:1)`. Do not point those fields at a Notion file attachment: attachment URLs are signed and expire in about an hour, so anything rendering from them breaks between sessions.

Filenames above already match the `Slug` field in the database, so the mapping back into Notion is mechanical.
