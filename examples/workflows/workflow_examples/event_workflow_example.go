package workflows

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/louloulin/gostra/pkg/workflow"
)

// RunEventWorkflowExample 运行事件驱动工作流示例
func RunEventWorkflowExample() {
	// 创建临时目录存储工作流状态
	tempDir, err := os.MkdirTemp("", "workflow-states")
	if err != nil {
		log.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建状态存储
	stateStore, err := workflow.NewFileStateStore(tempDir)
	if err != nil {
		log.Fatalf("Failed to create state store: %v", err)
	}

	// 创建事件工作流
	orderWorkflow, err := workflow.NewEventWorkflow(workflow.EventWorkflowOptions{
		ID:          "order-workflow",
		Name:        "订单处理工作流",
		Description: "处理订单从创建到完成的整个流程",
		StartStepID: "create-order",
		StateStore:  stateStore,
	})

	if err != nil {
		log.Fatalf("Failed to create workflow: %v", err)
	}

	// 创建步骤1: 创建订单
	createOrderStep := orderWorkflow.CreateStep(
		"create-order",
		"创建订单",
		"初始化新的订单",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			inputData, ok := data.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid input data format")
			}

			// 获取输入参数
			productID, _ := inputData["productId"].(string)
			quantity, _ := inputData["quantity"].(float64)
			userID, _ := inputData["userId"].(string)

			fmt.Printf("创建订单: 产品 %s, 数量 %.0f, 用户 %s\n",
				productID, quantity, userID)

			// 返回订单信息作为结果
			return map[string]interface{}{
				"orderId":     "ORD-" + time.Now().Format("20060102-150405"),
				"productId":   productID,
				"quantity":    quantity,
				"userId":      userID,
				"status":      "created",
				"createdAt":   time.Now(),
				"totalAmount": 99.99, // 假设价格
			}, nil
		},
		map[string]string{
			"*": "payment-pending", // 任何事件都转到支付等待
		},
	)

	// 创建步骤2: 等待支付
	paymentPendingStep := orderWorkflow.CreateStep(
		"payment-pending",
		"等待支付",
		"等待用户完成订单支付",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			orderId := stepData["orderId"].(string)
			fmt.Printf("等待订单 %s 的支付\n", orderId)

			return map[string]interface{}{
				"paymentStatus": "pending",
			}, nil
		},
		map[string]string{
			"payment-completed": "fulfill-order", // 支付完成事件
			"cancel-order":      "cancel-order",  // 取消订单事件
		},
	)

	// 创建步骤3: 履行订单
	fulfillOrderStep := orderWorkflow.CreateStep(
		"fulfill-order",
		"履行订单",
		"处理订单并准备发货",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			orderId := stepData["orderId"].(string)
			fmt.Printf("履行订单 %s\n", orderId)

			// 模拟订单处理
			time.Sleep(1 * time.Second)

			return map[string]interface{}{
				"status":         "fulfilled",
				"fulfilledAt":    time.Now(),
				"trackingNumber": "TRK-" + time.Now().Format("20060102"),
			}, nil
		},
		map[string]string{
			"*": "shipping", // 任何事件都转到发货
		},
	)

	// 创建步骤4: 发货
	shippingStep := orderWorkflow.CreateStep(
		"shipping",
		"发货",
		"订单发货处理",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			orderId := stepData["orderId"].(string)
			trackingNumber := stepData["trackingNumber"].(string)
			fmt.Printf("发货订单 %s, 跟踪号 %s\n", orderId, trackingNumber)

			return map[string]interface{}{
				"status":           "shipped",
				"shippedAt":        time.Now(),
				"estimatedArrival": time.Now().Add(72 * time.Hour),
			}, nil
		},
		map[string]string{
			"delivery-confirmed": "complete-order", // 确认送达事件
		},
	)

	// 创建步骤5: 完成订单
	completeOrderStep := orderWorkflow.CreateStep(
		"complete-order",
		"完成订单",
		"标记订单为完成状态",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			orderId := stepData["orderId"].(string)
			fmt.Printf("完成订单 %s\n", orderId)

			return map[string]interface{}{
				"status":      "completed",
				"completedAt": time.Now(),
				"feedback":    "NA",
			}, nil
		},
		nil, // 工作流结束
	)

	// 创建步骤6: 取消订单
	cancelOrderStep := orderWorkflow.CreateStep(
		"cancel-order",
		"取消订单",
		"处理订单取消",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			orderId := stepData["orderId"].(string)
			fmt.Printf("取消订单 %s\n", orderId)

			return map[string]interface{}{
				"status":       "cancelled",
				"cancelledAt":  time.Now(),
				"cancelReason": "用户请求",
			}, nil
		},
		nil, // 工作流结束
	)

	// 添加步骤到工作流
	if err := orderWorkflow.AddStep(createOrderStep); err != nil {
		log.Fatalf("Failed to add create order step: %v", err)
	}
	if err := orderWorkflow.AddStep(paymentPendingStep); err != nil {
		log.Fatalf("Failed to add payment pending step: %v", err)
	}
	if err := orderWorkflow.AddStep(fulfillOrderStep); err != nil {
		log.Fatalf("Failed to add fulfill order step: %v", err)
	}
	if err := orderWorkflow.AddStep(shippingStep); err != nil {
		log.Fatalf("Failed to add shipping step: %v", err)
	}
	if err := orderWorkflow.AddStep(completeOrderStep); err != nil {
		log.Fatalf("Failed to add complete order step: %v", err)
	}
	if err := orderWorkflow.AddStep(cancelOrderStep); err != nil {
		log.Fatalf("Failed to add cancel order step: %v", err)
	}

	// 启动工作流
	orderData := map[string]interface{}{
		"productId": "PROD-123",
		"quantity":  2.0,
		"userId":    "USER-456",
	}

	instanceID, err := orderWorkflow.StartWorkflow(context.Background(), orderData)
	if err != nil {
		log.Fatalf("Failed to start workflow: %v", err)
	}

	fmt.Printf("工作流实例已创建: %s\n", instanceID)

	// 等待工作流暂停在支付步骤
	time.Sleep(500 * time.Millisecond)

	// 检查工作流状态
	state, err := orderWorkflow.GetState(instanceID)
	if err != nil {
		log.Fatalf("Failed to get workflow state: %v", err)
	}

	fmt.Printf("工作流状态: %s, 当前步骤: %s\n", state.Status, state.CurrentStepID)
	fmt.Printf("订单ID: %s\n", state.Results["orderId"])

	// 模拟用户完成支付
	fmt.Println("\n用户完成支付...")
	err = orderWorkflow.HandleEvent(context.Background(), instanceID, "payment-completed", map[string]interface{}{
		"paymentId":     "PAY-789",
		"paymentMethod": "credit_card",
		"amount":        99.99,
		"paidAt":        time.Now(),
	})

	if err != nil {
		log.Fatalf("Failed to handle payment event: %v", err)
	}

	// 等待工作流执行到发货步骤
	time.Sleep(2 * time.Second)

	// 再次检查工作流状态
	state, err = orderWorkflow.GetState(instanceID)
	if err != nil {
		log.Fatalf("Failed to get workflow state: %v", err)
	}

	fmt.Printf("\n工作流状态: %s, 当前步骤: %s\n", state.Status, state.CurrentStepID)
	fmt.Printf("跟踪号: %s\n", state.Results["trackingNumber"])

	// 模拟确认送达
	fmt.Println("\n确认订单送达...")
	err = orderWorkflow.HandleEvent(context.Background(), instanceID, "delivery-confirmed", map[string]interface{}{
		"deliveredAt":   time.Now(),
		"receivedBy":    "客户本人",
		"deliveryNotes": "放在门口",
	})

	if err != nil {
		log.Fatalf("Failed to handle delivery event: %v", err)
	}

	// 等待工作流完成
	time.Sleep(500 * time.Millisecond)

	// 最终检查工作流状态
	state, err = orderWorkflow.GetState(instanceID)
	if err != nil {
		log.Fatalf("Failed to get workflow state: %v", err)
	}

	fmt.Printf("\n最终工作流状态: %s\n", state.Status)
	fmt.Println("订单处理结果:")
	for k, v := range state.Results {
		fmt.Printf("  %s: %v\n", k, v)
	}
}

// RunCancelOrderExample 演示取消订单的情况
func RunCancelOrderExample() {
	stateStore := workflow.NewInMemoryStateStore()

	// 创建工作流
	orderWorkflow, err := workflow.NewEventWorkflow(workflow.EventWorkflowOptions{
		ID:          "cancel-order-workflow",
		Name:        "取消订单工作流示例",
		Description: "演示订单取消流程",
		StartStepID: "create-order",
		StateStore:  stateStore,
	})

	if err != nil {
		log.Fatalf("Failed to create workflow: %v", err)
	}

	// 创建并添加步骤 (简化版)
	orderWorkflow.AddStep(orderWorkflow.CreateStep(
		"create-order", "创建订单", "初始化订单",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			fmt.Println("创建新订单")
			return map[string]interface{}{
				"orderId": "ORD-CANCEL-TEST",
				"status":  "created",
			}, nil
		},
		map[string]string{"*": "payment-pending"},
	))

	orderWorkflow.AddStep(orderWorkflow.CreateStep(
		"payment-pending", "等待支付", "等待用户支付",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			fmt.Println("等待支付")
			return map[string]interface{}{
				"paymentStatus": "pending",
			}, nil
		},
		map[string]string{
			"payment-completed": "fulfill-order",
			"cancel-order":      "cancel-order",
		},
	))

	orderWorkflow.AddStep(orderWorkflow.CreateStep(
		"fulfill-order", "履行订单", "处理订单",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			fmt.Println("处理订单")
			return map[string]interface{}{
				"status": "fulfilled",
			}, nil
		},
		nil,
	))

	orderWorkflow.AddStep(orderWorkflow.CreateStep(
		"cancel-order", "取消订单", "处理订单取消",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			fmt.Println("取消订单")
			return map[string]interface{}{
				"status":       "cancelled",
				"cancelReason": "用户请求",
			}, nil
		},
		nil,
	))

	// 启动工作流
	instanceID, _ := orderWorkflow.StartWorkflow(context.Background(), map[string]interface{}{
		"productId": "PROD-CANCEL-TEST",
	})

	fmt.Printf("取消测试工作流实例已创建: %s\n", instanceID)

	// 等待工作流暂停
	time.Sleep(200 * time.Millisecond)

	// 发送取消事件
	fmt.Println("\n用户请求取消订单...")
	orderWorkflow.HandleEvent(context.Background(), instanceID, "cancel-order", nil)

	// 等待工作流完成
	time.Sleep(200 * time.Millisecond)

	// 检查最终状态
	state, _ := orderWorkflow.GetState(instanceID)
	fmt.Printf("\n取消订单最终状态: %s\n", state.Status)
	fmt.Printf("订单状态: %s\n", state.Results["status"])
	fmt.Printf("取消原因: %s\n", state.Results["cancelReason"])
}

func EventWorkflowMain() {
	fmt.Println("=== 运行订单处理工作流示例 ===")
	RunEventWorkflowExample()

	fmt.Println("\n\n=== 运行订单取消工作流示例 ===")
	RunCancelOrderExample()
}
