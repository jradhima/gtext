package main

type Command struct {
	key    rune
	name   string
	desc   string
	action func()
}

type CommandRegistry struct {
	cmds  map[rune]Command
	order []rune
}

func (cr *CommandRegistry) register(cmd Command) {
	if cr.cmds == nil {
		cr.cmds = make(map[rune]Command)
	}
	if _, exists := cr.cmds[cmd.key]; !exists {
		cr.order = append(cr.order, cmd.key)
	}
	cr.cmds[cmd.key] = cmd
}

func (cr *CommandRegistry) execute(e *Editor, key rune) bool {
	if cmd, ok := cr.cmds[key]; ok {
		cmd.action()
		return true
	}
	return false
}
