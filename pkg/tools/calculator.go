//   pkg/tools/calculator.go

package tools

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"

	"github.com/picoclaw/pkg/llm"
)

// CalculatorTool evaluates simple arithmetic expressions.
type CalculatorTool struct{}

func (t *CalculatorTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	expr, _ := params["expression"].(string)
	if expr == "" {
		return nil, fmt.Errorf("'expression' is required")
	}
	result, err := evalExpr(expr)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func evalExpr(expr string) (float64, error) {
	node, err := parser.ParseExpr(expr)
	if err != nil {
		return 0, err
	}
	return evalNode(node)
}

func evalNode(node ast.Expr) (float64, error) {
	switch n := node.(type) {
	case *ast.BasicLit:
		if n.Kind == token.INT || n.Kind == token.FLOAT {
			return strconv.ParseFloat(n.Value, 64)
		}
		return 0, fmt.Errorf("unsupported literal")
	case *ast.ParenExpr:
		return evalNode(n.X)
	case *ast.BinaryExpr:
		left, err := evalNode(n.X)
		if err != nil {
			return 0, err
		}
		right, err := evalNode(n.Y)
		if err != nil {
			return 0, err
		}
		switch n.Op {
		case token.ADD:
			return left + right, nil
		case token.SUB:
			return left - right, nil
		case token.MUL:
			return left * right, nil
		case token.QUO:
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			return left / right, nil
		default:
			return 0, fmt.Errorf("unsupported operator: %s", n.Op)
		}
	case *ast.UnaryExpr:
		val, err := evalNode(n.X)
		if err != nil {
			return 0, err
		}
		if n.Op == token.SUB {
			return -val, nil
		}
		return val, nil
	default:
		return 0, fmt.Errorf("unsupported expression type")
	}
}

func (t *CalculatorTool) Describe() llm.ToolDef {
	return llm.ToolDef{
		Name:        "calculator",
		Description: "Evaluate a simple arithmetic expression (supports +, -, *, /, parentheses).",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"expression": map[string]interface{}{"type": "string"},
			},
			"required": []string{"expression"},
		},
	}
}