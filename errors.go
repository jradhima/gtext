package main

type gtextError string

func (g gtextError) Error() string {
	return string(g)
}
