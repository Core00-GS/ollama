package parser

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

type Command struct {
	Name string
	Args string
	bytes.Buffer
}

func Parse(r io.Reader) ([]Command, error) {
	var cmds []Command
	var errs []error
	parseCommands(r)(func(cmd Command, err error) bool {
		if err != nil {
			errs = append(errs, err)
			return false
		}

		cmds = append(cmds, cmd)
		return true
	})

	if len(errs) > 0 {
		return nil, errs[0]
	}

	for _, cmd := range cmds {
		if cmd.Name == "model" {
			return cmds, nil
		}
	}

	return nil, errors.New("no FROM line")
}

const (
	stateName = iota
	stateArgs
	stateMultiline
	stateParameter
	stateMessage

	stateUnknown
)

func parseCommands(r io.Reader) func(func(Command, error) bool) {
	return func(yield func(Command, error) bool) {
		var c Command
		var b bytes.Buffer

		var quotes int

		s := stateName
		br := bufio.NewReader(r)
		for {
			r, _, err := br.ReadRune()
			if errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				yield(Command{}, err)
				return
			}

			if _, err := c.WriteRune(r); err != nil {
				yield(Command{}, err)
				return
			}

			// trim leading whitespace
			if (space(r) || newline(r)) && b.Len() == 0 {
				continue
			}

			switch s {
			case stateName, stateParameter:
				if alpha(r) || number(r) {
					if _, err := b.WriteRune(r); err != nil {
						yield(Command{}, err)
						return
					}
				} else if space(r) {
					c.Name = strings.ToLower(b.String())
					b.Reset()

					if c.Name == "from" {
						c.Name = "model"
					}

					switch c.Name {
					case "parameter":
						s = stateParameter
					case "message":
						s = stateMessage
					default:
						s = stateArgs
					}
				} else if newline(r) {
					yield(Command{}, fmt.Errorf("missing value for [%s]", b.String()))
					return
				}
			case stateArgs:
				if r == '"' && b.Len() == 0 {
					quotes++
					s = stateMultiline
				} else if newline(r) {
					c.Args = b.String()
					b.Reset()
					if !yield(c, nil) {
						return
					}

					c = Command{}
					s = stateName
				} else {
					if _, err := b.WriteRune(r); err != nil {
						yield(Command{}, err)
						return
					}
				}
			case stateMultiline:
				if r == '"' && b.Len() == 0 {
					quotes++
					continue
				} else if r == '"' {
					if quotes--; quotes == 0 {
						c.Args = b.String()
						b.Reset()
						if !yield(c, nil) {
							return
						}

						c = Command{}
						s = stateName
					}

					continue
				} else {
					if _, err := b.WriteRune(r); err != nil {
						yield(Command{}, err)
						return
					}
				}
			case stateMessage:
				if space(r) && !isValidRole(b.String()) {
					yield(Command{}, errors.New("role must be one of \"system\", \"user\", or \"assistant\""))
					return
				} else if space(r) {
					if _, err := b.WriteRune(':'); err != nil {
						yield(Command{}, err)
						return
					}
					s = stateArgs
				}

				if _, err := b.WriteRune(r); err != nil {
					yield(Command{}, err)
					return
				}
			}
		}
	}
}

func alpha(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

func number(r rune) bool {
	return r >= '0' && r <= '9'
}

func space(r rune) bool {
	return r == ' ' || r == '\t'
}

func newline(r rune) bool {
	return r == '\r' || r == '\n'
}

func isValidRole(role string) bool {
	return role == "system" || role == "user" || role == "assistant"
}
