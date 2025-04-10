// 简化版工作流示例，兼容当前API实现
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/louloulin/gostra/pkg/workflow"
)

func main() {
	// 创建工作流步骤
	steps := []*workflow.Step{
		{
			ID:          "start",
			Name:        "Starting Step",
			Description: "The first step in the workflow",
			Execute: func(ctx context.Context, input map[string]interface{}, w *workflow.Workflow) (map[string]interface{}, error) {
				fmt.Println("Executing starting step...")
				return map[string]interface{}{
					"step1_result": "Initial data processed",
				}, nil
			},
			Next: []string{"process"},
		},
		{
			ID:          "process",
			Name:        "Processing Step",
			Description: "Process the data",
			Execute: func(ctx context.Context, input map[string]interface{}, w *workflow.Workflow) (map[string]interface{}, error) {
				fmt.Println("Processing data...")

				// 获取上一步结果
				stepResult, ok := input["step1_result"].(string)
				if !ok {
					return nil, fmt.Errorf("invalid input from previous step")
				}

				return map[string]interface{}{
					"step2_result": fmt.Sprintf("Processed: %s", stepResult),
				}, nil
			},
			Next: []string{"finish"},
		},
		{
			ID:          "finish",
			Name:        "Final Step",
			Description: "Finalize the workflow",
			Execute: func(ctx context.Context, input map[string]interface{}, w *workflow.Workflow) (map[string]interface{}, error) {
				fmt.Println("Finalizing workflow...")

				// 获取上一步结果
				stepResult, ok := input["step2_result"].(string)
				if !ok {
					return nil, fmt.Errorf("invalid input from previous step")
				}

				return map[string]interface{}{
					"final_result": fmt.Sprintf("Final result: %s", stepResult),
				}, nil
			},
		},
	}

	// 创建工作流选项
	workflowOptions := &workflow.WorkflowOptions{
		Name:        "Simple Sequential Workflow",
		Description: "A basic workflow with sequential steps",
		Steps:       steps,
		StartStep:   "start",
		Metadata: map[string]interface{}{
			"version": "1.0.0",
			"type":    "example",
		},
	}

	// 创建工作流
	myWorkflow, err := workflow.NewWorkflow(workflowOptions)
	if err != nil {
		log.Fatalf("Failed to create workflow: %v", err)
	}

	fmt.Println("Workflow created successfully. Starting execution...")

	// 设置开始时间以测量性能
	startTime := time.Now()

	// 运行工作流
	result, err := myWorkflow.Run(context.Background(), nil)
	if err != nil {
		log.Fatalf("Error running workflow: %v", err)
	}

	// 计算执行时间
	executionTime := time.Since(startTime)

	// 打印结果
	fmt.Printf("\nWorkflow completed in %v\n", executionTime)
	fmt.Printf("Final result: %s\n", result["final_result"])

	fmt.Println("\nAll workflow results:")
	for k, v := range result {
		fmt.Printf("- %s: %v\n", k, v)
	}
}
