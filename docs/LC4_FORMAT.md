# LC4 / PRV file format (HOMAG CADmatic 4.5, SAWTEQ S-200)

Reverse engineered from the 156 runs in `test/`. The Python reader/writer in `lc4/` parses
**all 156 `.LC4` and 156 `.prv` samples and writes them back byte-identical**
(`python3 -m unittest tests.test_lc4`).

The machine reads runs from `D:\machine1\prod_data\Cadv51\data\Runs` (`instruction.txt`).
Every run is a pair: `<name>.LC4` (the cutting patterns) and `<name>.prv` (run summary).

## 1. Encoding

A stream of tagged values, little endian. Every value starts with a 16-bit tag:

| tag | payload | meaning |
|---|---|---|
| `01 00` | 1 byte (0/1) | bool; also "present / null" flag before optional objects |
| `05 00` | u16 | start of an object, u16 = class version (no length, no end marker) |
| `06 00` | i32 | integer |
| `08 00` | i64 | 64-bit integer (only inside decimals) |
| `0a 00` | 4 bytes | unknown 4-byte value (always 0) |
| `0b 00` | f64 | double (area statistics) |
| `0d 00` | bool + `0e 00` + int(len) + bytes | string (cp1252). `0d 00 01 00 00` = null string |
| `0f 00` | `05 00 01 00 08 00 <i64>` | decimal = i64 / 100000 (all lengths in mm) |
| `10 00`…`30 00` | i32 | enum values (tag identifies the enum) |

A list is `01 00 01 | 05 00 01 00 | 06 00 <count>` followed by `count` items.
The format has no field names; the layout below is fixed by position.

## 2. LC4 layout

```
bool(1) struct(12)
  string  run name                      (= file name)
  string  "manually generated"
  list    (always empty)
  list    boards[n]
  list    (always empty)
  list    patterns[m]
  trailer (fixed bytes, see lc4/template.py)
```

**Board** — `bool(1) struct(1) struct(4)`, 3 strings (empty), decimal length + bool,
decimal width + bool, decimal thickness + bool, int qty(1), int (used count), enum 0x13,
bool, empty list, bool, raw4, 2 strings, raw4, enum 0x30. One board per pattern, same order.

**Pattern** = root node (`struct(9) struct(9)` …) followed by: decimal board length + bool,
int **quantity (number of boards cut with this pattern)**, int 1, string, `struct(1)` enum 0x10 + bool,
list of 110 saw parameters (`struct(2)` name, value, bool — identical in all samples),
then statistics: int, bool, double waste area, double board area, int rest count,
double rest area, double kerf area, int part count, double part area, double waste %,
enum 0x27, 4 ints, int cut count, decimal total cut length + bool, int, int 4, string, int, bool, int, string.

**Node** (`bool(1) struct(9)`):

| # | field | notes |
|---|---|---|
| 1 | string name | `Anschnitt`, `Schnitt`, `Streifen`, `Teil`, `Rest`, root: `00001`… |
| 2 | string label | `''`, or `'Copy'` for repeated items |
| 3 | decimal size + bool | mm, measured in the direction its parent is divided |
| 4 | int qty | repeat count; copies that follow have qty 0 |
| 5 | enum 0x11 type | 1 root, 2 cut (Schnitt, size = kerf 4.4), 3 strip, 4 trim/waste (Anschnitt), 10 rest (offcut), 11 part |
| 6 | enum 0x12 | 1 = root / strip containing strips, 0 = strip of parts / part, 4 = cut & trim |
| 7 | `struct(1)` int n + n items | root only: `struct(3)` …, enum 0x14 = 3, int = pattern index |
| 8 | list | always empty |
| 9 | list children | the next cutting stage |
| 10 | bool, enum 0x26, int, bool, int -1, int -1, 2 empty lists, string '', int -1, string '-1' | constant |

### Geometry

The root's size is the board width W and the pattern length the board length L.
Children of the root divide L, their children divide W, and so on (directions alternate).
Most patterns (619/716) start with a single full-length `Streifen` (size L), so the
first real cuts divide W into strips that run along the whole board length.

Sizes follow this rule (holds for the large majority of the sample nodes):

```
extent = leading trim + Σ items + kerf × (number of items − 1) + trailing trim
```

i.e. a leading `Anschnitt` already contains the kerf of the cut that follows it, and so does a trailing
waste `Anschnitt`. Edge trims in the samples are 15 mm on stages 1–2 and 10 mm on recuts; the kerf is 4.4 mm.

Repeated identical items are written once with qty = n followed by n−1 copies (`label='Copy'`,
qty = 0, second node flag = true), each separated by a `Schnitt`.

## 3. PRV layout

```
bool(1) struct(6) string name, string "manually generated",
int patterns, int boards, int 0, int 0, int total boards (Σ pattern qty), int status,
enum 0x25, bool, bool, bool, int 0, int 0, int 0
```
