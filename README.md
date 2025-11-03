# gtext

A minimal terminal text editor written in Go — inspired by the **kilo** editor.  
Tested on macOS, should work on Linux and WSL2.  

---

## Features

- Line-based text editing
- Save (`Ctrl-S`) and Quit (`Ctrl-Q`)
- Cut / Copy / Paste lines (`Ctrl-X`, `Ctrl-C`, `Ctrl-V`)
- Search mode (`Ctrl-F`)
- Auto-load configuration from `~/.gtext.conf`

---

## Build Instructions

### macOS / Linux / WSL2
Make sure Go (1.23+) is installed.

```bash
git clone https://github.com/jradhima/gtext.git
cd gtext
go build -o gtext
````

You can then run:

```bash
./gtext myfile.txt     # Open or create a file
./gtext                # Start with a new document "untitled.txt"
./gtext config         # Interactive setup of configuration
```

---

## Configuration

The editor reads settings from `~/.gtext.conf`.
To initialize or edit this file interactively:

```bash
./gtext config
```

Example config file:

```ini
# gtext config file
show_line_numbers=true
expand_tabs=false
tab_size=4
scroll_margin=5
```

---

## Key Commands

| Key         | Action               |
| ----------- | -------------------- |
| `Ctrl-S`    | Save file            |
| `Ctrl-Q`    | Quit editor          |
| `Ctrl-F`    | Toggle find mode     |
| `Ctrl-X`    | Cut current line     |
| `Ctrl-C`    | Copy current line    |
| `Ctrl-V`    | Paste copied lines   |
| Arrow keys  | Move cursor          |
| `Return`    | New line             |
| `Backspace` | Delete character     |
| `Tab`       | Insert tab or spaces |

---

## License

MIT License © 2025
