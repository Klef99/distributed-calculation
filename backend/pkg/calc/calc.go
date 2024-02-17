package calc

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

type Operation struct {
	ExpressionID string
	Operator     string
	V1           interface{}
	V2           interface{}
	OperationID  string
	ParentID     string
	Left         bool
	Status       int
}

func (op Operation) Task() float64 {
	operTimeout := map[string]time.Duration{"+": time.Second * 2, "-": time.Second * 1, "*": time.Second * 3, "/": time.Second * 4}
	time.Sleep(operTimeout[op.Operator])
	switch op.Operator {
	case "+":
		return op.V1.(float64) + op.V2.(float64)
	case "-":
		return op.V1.(float64) - op.V2.(float64)
	case "/":
		return op.V1.(float64) / op.V2.(float64)
	case "*":
		return op.V1.(float64) * op.V2.(float64)
	}
	panic("unreachable operator")
}

func precedence(operator rune) int {
	switch operator {
	case '+', '-':
		return 1
	case '*', '/':
		return 2
	}
	return 0
}

func isDigit(char rune) bool {
	return unicode.IsDigit(char)
}

func infixToPostfix(infix string) string {
	var postfix strings.Builder
	var stack []rune
	var number strings.Builder
	infix = strings.TrimSpace(infix)

	for _, char := range infix {
		if isDigit(char) {
			number.WriteRune(char)
		} else {
			if number.Len() > 0 {
				postfix.WriteString(number.String())
				postfix.WriteRune(' ')
				number.Reset()
			}

			switch char {
			case '(':
				stack = append(stack, char)
			case ')':
				for len(stack) > 0 && stack[len(stack)-1] != '(' {
					postfix.WriteRune(stack[len(stack)-1])
					postfix.WriteRune(' ')
					stack = stack[:len(stack)-1]
				}
				stack = stack[:len(stack)-1]
			default:
				for len(stack) > 0 && precedence(stack[len(stack)-1]) >= precedence(char) {
					postfix.WriteRune(stack[len(stack)-1])
					postfix.WriteRune(' ')
					stack = stack[:len(stack)-1]
				}
				stack = append(stack, char)
			}
		}
	}

	if number.Len() > 0 {
		postfix.WriteString(number.String())
		postfix.WriteRune(' ')
	}

	for len(stack) > 0 {
		postfix.WriteRune(stack[len(stack)-1])
		postfix.WriteRune(' ')
		stack = stack[:len(stack)-1]
	}

	return strings.TrimSpace(postfix.String())
}

func IsOperation(t interface{}) bool {
	switch t.(type) {
	case Operation:
		return true
	}
	return false
}

func TransformExpressionToStack(expressionID, expression string) []Operation {
	tokens := strings.Split(infixToPostfix(expression), " ")
	opers := make([]interface{}, 0)
	tasks := make([]Operation, 0)
	for i := 0; i < len(tokens); i++ {
		if v, err := strconv.ParseFloat(tokens[i], 64); err == nil {
			opers = append(opers, v)
		} else {
			if len(opers) < 2 {
				panic("dont much values")
			}
			v1 := opers[len(opers)-2]
			v2 := opers[len(opers)-1]
			opers = opers[:len(opers)-2]
			task := Operation{Operator: tokens[i], OperationID: uuid.New().String(), ExpressionID: expressionID, ParentID: expressionID, Status: 0}
			if IsOperation(v1) {
				v1, _ := v1.(Operation)
				v1.ParentID = task.OperationID
				v1.Left = true
				for i := 0; i < len(tasks); i++ {
					if v1.OperationID == tasks[i].OperationID {
						tasks[i] = v1
						break
					}
				}
			}
			if IsOperation(v2) {
				v2, _ := v2.(Operation)
				v2.ParentID = task.OperationID
				v2.Left = false
				for i := 0; i < len(tasks); i++ {
					if v2.OperationID == tasks[i].OperationID {
						tasks[i] = v2
						break
					}
				}
			}
			task.V1 = v1
			task.V2 = v2
			opers = append(opers, task)
			tasks = append(tasks, task)
		}
	}
	for i := 0; i < len(tasks); i++ {
		if IsOperation(tasks[i].V1) {
			tasks[i].V1 = nil
		}
		if IsOperation(tasks[i].V2) {
			tasks[i].V2 = nil
		}
	}
	return tasks
}

// Очищение и валидация выражения
func ValidExpression(expression string) (string, error) {
	re := regexp.MustCompile(`[^0-9+\-*/() ]`)
	res := re.ReplaceAllString(strings.ReplaceAll(expression, " ", ""), "")
	scb := []rune{}
	for i := 0; i < len(res); i++ {
		if res[i] == '{' {
			scb = append(scb, '{')
		} else if res[i] == '}' {
			scb = scb[:len(scb)-1]
		}
	}
	if len(scb) != 0 {
		return "", fmt.Errorf("invalid count of brackets")
	}
	return res, nil
}
