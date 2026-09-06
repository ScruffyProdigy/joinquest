#!/usr/bin/env bash
# Copies the winning version of each asset from docs/catalog-art-raw/ into
# docs/catalog-art/ under slug names. Raw folder is left untouched.
# Final selections after draft 3 (2026-08-30). All 40 assets settled.
set -euo pipefail
SRC="$(cd "$(dirname "$0")" && pwd)/catalog-art-raw"
DST="$(cd "$(dirname "$0")" && pwd)/catalog-art"
mkdir -p "$DST"

# ---- HEROES (4:3) ----
cp "$SRC/Geometric Letter Tile Grid.png"                 "$DST/word-hunt-hero.png"          # draft 2
cp "$SRC/Four colorful buzzers, one pressed.png"         "$DST/know-better-hero.png"        # draft 2
cp "$SRC/ChatGPT Image Aug 30, 2026 at 11_57_15 AM.png"  "$DST/two-camps-hero.png"
cp "$SRC/ChatGPT Image Aug 30, 2026 at 11_57_34 AM.png"  "$DST/rpslr-hero.png"
cp "$SRC/Three-Compartment Ship Deck Cutaway.png"        "$DST/vent-crew-hero.png"          # draft 3
cp "$SRC/ChatGPT Image Aug 30, 2026 at 12_05_05 PM.png"  "$DST/tell-nothing-hero.png"
cp "$SRC/Snowy Meadow Village Well.png"                  "$DST/salt-the-well-hero.png"
cp "$SRC/ChatGPT Image Aug 30, 2026 at 12_10_13 PM.png"  "$DST/whisper-sketch-hero.png"
cp "$SRC/Many hands, one canvas.png"                     "$DST/handiwork-hero.png"          # draft 2
cp "$SRC/Fast marker scribble with stopwatch.png"        "$DST/inkbolt-hero.png"
cp "$SRC/Abandoned Mop Under the Striplight.png"         "$DST/skeleton-staff-hero.png"     # draft 2
cp "$SRC/Candlelit Table of Five Houses.png"             "$DST/kettle-and-crown-hero.png"
cp "$SRC/Violet Rift Across Ruined Battlements.png"      "$DST/sunderline-hero.png"
cp "$SRC/Unbalanced Clockwork and Rootwork Scale.png"    "$DST/counterpoise-hero.png"
cp "$SRC/Four Toy Rally Cars Drift Through Dust.png"     "$DST/shale-run-hero.png"
cp "$SRC/Boxing Ring Clash in Ink.png"                   "$DST/standing-eight-hero.png"
cp "$SRC/Battle for the Bronze Bell.png"                 "$DST/siege-bell-hero.png"
cp "$SRC/Luminous Creatures in the Shadow Arena.png"     "$DST/crumbfall-hero.png"
cp "$SRC/Split mechanical lock cutaway.png"              "$DST/both-hands-hero.png"
cp "$SRC/Twin neon loops, one loosening.png"             "$DST/cinchpoint-hero.png"

# ---- ICONS (1:1) ----
cp "$SRC/Amber RKW tiles on navy.png"                    "$DST/word-hunt-icon.png"          # draft 2
cp "$SRC/Pressed Coral Buzzer with Impact Lines.png"     "$DST/know-better-icon.png"        # draft 3
cp "$SRC/Mid-Century Profile Face-Off.png"               "$DST/two-camps-icon.png"          # draft 2
cp "$SRC/ChatGPT Image Aug 30, 2026 at 11_57_22 AM.png"  "$DST/rpslr-icon.png"              # coral version
cp "$SRC/Six-Slat Amber Floor Vent.png"                  "$DST/vent-crew-icon.png"          # draft 3
cp "$SRC/ChatGPT Image Aug 30, 2026 at 12_05_11 PM.png"  "$DST/tell-nothing-icon.png"
cp "$SRC/Paper-cut snowy village well.png"               "$DST/salt-the-well-icon.png"
cp "$SRC/Wobbly cat doodle on notebook paper.png"        "$DST/whisper-sketch-icon.png"
cp "$SRC/Five hands, one shared canvas.png"              "$DST/handiwork-icon.png"
cp "$SRC/Bold felt-tip spiral with blue accent.png"      "$DST/inkbolt-icon.png"
cp "$SRC/Sickly Green Striplight Glow.png"               "$DST/skeleton-staff-icon.png"     # draft 2
cp "$SRC/Golden Crowned Heraldic Shield.png"             "$DST/kettle-and-crown-icon.png"   # draft 2
cp "$SRC/Armoured shield with violet fracture.png"       "$DST/sunderline-icon.png"         # draft 2
cp "$SRC/Tipped balance of gears and roots.png"          "$DST/counterpoise-icon.png"       # draft 2
cp "$SRC/Toy Rally Car Bursting Through Dust.png"        "$DST/shale-run-icon.png"
cp "$SRC/Red glove, full-force punch.png"                "$DST/standing-eight-icon.png"
cp "$SRC/Dustlit Giant Bronze Bell.png"                  "$DST/siege-bell-icon.png"
cp "$SRC/Hooked Predator Over a Lime Orb.png"            "$DST/crumbfall-icon.png"          # draft 2
cp "$SRC/Split teal mechanical padlock.png"              "$DST/both-hands-icon.png"         # draft 2
cp "$SRC/Neon cord knot on black.png"                    "$DST/cinchpoint-icon.png"

# ---- spare ----
cp "$SRC/ChatGPT Image Aug 30, 2026 at 11_57_39 AM.png"  "$DST/_rpslr-icon-alt-mint.png"

echo "Copied $(ls "$DST" | wc -l | tr -d ' ') files into $DST"
echo "All 40 assets final (20 heroes + 20 icons), plus 1 spare."
