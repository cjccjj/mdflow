# A Short Tour of Markdown

Markdown is plain text: readable in the source, pretty
when rendered. 

It is the lingua franca of chat apps, forums, and docs.
Chances are you have written some already, unknowingly.

Written by hand or streamed from a model, it renders
the same way — one line at a time, first to last.

## Inline text

One style at a time, no hurry:

- **bold** for emphasis
- *italic* for titles and notes
- ~~strikethrough~~ for edits
- ==highlighted== for key results
- `code` for commands and paths
- H₂O and x² for science
- a [link](https://example.com) stays clickable with its URL tucked away

One quiet extra:

- entities: &copy;&nbsp;&amp;&nbsp;&hearts; just work

Punctuation is plain text — commas, dashes, and semicolons
are nothing special. Two trailing spaces make a hard break  
and jump to a fresh line without ending the paragraph.

## Deep headings

### Third level

Sub-headings get a style of their own, easy on the eyes.

#### Fourth level

Even deeper — the ladder stays readable all the way down.

## Lists

Styles live inside items, not just around them:

1. **install** the deps, then *verify* the version
2. ==highlight== the key steps: `make check`
3. math stays tidy: E = mc², H₂O — small and styled
4. read the [docs](https://example.com) for details
   - nested bullets carry _italics_ and ~~strikethrough~~
   - 日本語や中文もリストに混ざります 🎌 ただの文字列
5. ~~drop~~ the old version, ==ship== the new one

## Quotes

> **Bold** opens, *italic* sets mood, `code` quotes verbatim.
> _Italic_ labels words, ~~strike~~ marks the edits.

> ==Highlight== the takeaway, link the
> [source](https://example.com) — math fits: E = mc², H₂O.

> They nest, too:
>> 内側は静かに、**でも決して消えない** — quiet, never lost.
引用に日本語・中文・絵文字もOK：📖 今日の一言。

## Code

Code blocks show a language badge and their own palette.

```python
# Keep the report readable while the stream is still arriving.
from dataclasses import dataclass

@dataclass
class Planet:
    name: str
    moons: int

def describe(planet: Planet) -> str:
    label = f"{planet.name}: {planet.moons} moon(s)"
    if planet.moons > 3:
        return f"{label} — busy orbit"
    return f"{label} — quiet orbit"

planets = [Planet("Earth", 1), Planet("Jupiter", 95)]
for planet in planets:
    print(describe(planet))
```

## Solar census

A quick census of the solar system, aligned per column:

| name | diameter | note |
|:--|--:|:--|
| Mercury | 4,879 km | small |
| Venus | 12,104 km | hot |
| Earth | 12,742 km | home |
| Mars | 6,779 km | red |
| Jupiter | 142,984 km | **biggest** planet |
| Saturn | 120,536 km | rings of `ice` and rock — wider than the planet |
| Uranus | 51,118 km | rolls _sideways_ with a 98° tilt and long seasons |
| Neptune | 49,528 km | windiest — 2,100 km/h gusts, [record](https://example.com) |
| Pluto | 2,377 km | still a planet in our ==hearts== — 冥王星も好き ❤️ |

Wide cells wrap at the column edge, keeping their style.

## Unicode, CJK & emoji

Terminals speak many scripts, all plain Unicode:

- CJK: 中文・日本語・한국어 都按两格宽排布
- emoji: 🚀 🎨 🐛 🐚 — mostly two cells wide as well
- combining marks: e + ́ = é — zero width, no drift
- kaomoji (^▽^)／ and Cyrillic привет — width 1, plain text

## Checklist

- [x] build the library
- [x] write the tests
- [ ] write the demo[^1]
- [ ] cut tomorrow's release

> [!WARNING]
> Verify checksums, 
> before installing from any mirror.
---
One more line before bed, with a note or two[^1][^2].
[^1]: Footnote are collected while the stream runs, printed once it closes.
[^2]: A second footnote is fine; order is preserved.
