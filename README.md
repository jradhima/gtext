# gtext
A minimal text editor implemented in Go, inspired by the kilo editor.

## Usage
### Create or open a file
```
gtext <filename>
```
### Set up config
```
gtext config
```
### Commands supported
- Exit the editor with `Ctrl-Q` (does not save)
- Save the file with `Ctrl-S`
- Cut/Copy/Paste a line with `Ctrl-X|C|V`
- Enter/exit find mode with `Ctrl-F`

### Configuration
Pass configuration with the `.gtext.conf` file in your home directory.

Values currently supported and their defaults:
```
# gtext config file
show_line_numbers=true
expand_tabs=false
tab_size=4
