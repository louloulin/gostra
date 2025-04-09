package main

import (
	"context"
	"os"
	"testing"

	"github.com/yourusername/gostra/pkg/memory"
)

func TestPostgresVectorAdvancedFiltering(t *testing.T) {
	// 跳过测试，如果环境变量未设置
	pgConnStr := os.Getenv("POSTGRES_TEST_CONNECTION_STRING")
	if pgConnStr == "" {
		t.Skip("Skipping test: POSTGRES_TEST_CONNECTION_STRING not set")
	}

	// 创建上下文
	ctx := context.Background()

	// 创建PostgreSQL向量存储
	opts := memory.PostgresVectorOptions{
		ConnectionString: pgConnStr,
		TableName:        "test_vectors_advanced",
		VectorDimension:  3,
		BatchSize:        10,
	}

	store, err := memory.NewPostgresVectorStorage(opts)
	if err != nil {
		t.Fatalf("Failed to create PostgreSQL vector storage: %v", err)
	}
	defer store.Close()

	// 清除测试表
	err = store.Clear(ctx)
	if err != nil {
		t.Fatalf("Failed to clear vector storage: %v", err)
	}

	// 测试向量和复杂元数据
	vectors := []memory.Vector{
		{
			ID:     "article1",
			Values: []float32{1.0, 0.5, 0.1},
			Metadata: map[string]interface{}{
				"title":       "Introduction to Machine Learning",
				"author":      "John Smith",
				"published":   "2023",
				"word_count":  "5000",
				"categories":  []string{"ML", "AI", "Tutorial"},
				"rating":      "4.5",
				"has_code":    "true",
				"is_featured": "true",
				"keywords":    "machine learning, artificial intelligence, algorithms",
				"sections": map[string]interface{}{
					"introduction": map[string]interface{}{
						"length": "500",
						"topics": []string{"history", "basics"},
					},
					"algorithms": map[string]interface{}{
						"length":        "2000",
						"topics":        []string{"supervised", "unsupervised", "reinforcement"},
						"code_examples": "20",
					},
				},
			},
		},
		{
			ID:     "article2",
			Values: []float32{0.8, 0.9, 0.3},
			Metadata: map[string]interface{}{
				"title":       "Advanced Deep Learning",
				"author":      "Jane Doe",
				"published":   "2024",
				"word_count":  "8000",
				"categories":  []string{"DL", "AI", "Research"},
				"rating":      "4.8",
				"has_code":    "true",
				"is_featured": "true",
				"keywords":    "deep learning, neural networks, transformers",
				"sections": map[string]interface{}{
					"introduction": map[string]interface{}{
						"length": "600",
						"topics": []string{"history", "basics"},
					},
					"models": map[string]interface{}{
						"length":        "3000",
						"topics":        []string{"CNN", "RNN", "transformers"},
						"code_examples": "30",
					},
				},
			},
		},
		{
			ID:     "article3",
			Values: []float32{0.2, 0.3, 0.9},
			Metadata: map[string]interface{}{
				"title":       "Reinforcement Learning Tutorial",
				"author":      "Alex Brown",
				"published":   "2022",
				"word_count":  "3000",
				"categories":  []string{"RL", "AI", "Tutorial"},
				"rating":      "4.0",
				"has_code":    "true",
				"is_featured": "false",
				"keywords":    "reinforcement learning, q-learning, policy gradients",
				"sections": map[string]interface{}{
					"introduction": map[string]interface{}{
						"length": "400",
						"topics": []string{"basics", "applications"},
					},
					"algorithms": map[string]interface{}{
						"length":        "1500",
						"topics":        []string{"Q-learning", "SARSA", "A3C"},
						"code_examples": "15",
					},
				},
			},
		},
		{
			ID:     "article4",
			Values: []float32{0.6, 0.2, 0.7},
			Metadata: map[string]interface{}{
				"title":       "Machine Learning vs Statistics",
				"author":      "Mary Williams",
				"published":   "2021",
				"word_count":  "4000",
				"categories":  []string{"ML", "Statistics", "Comparison"},
				"rating":      "3.9",
				"has_code":    "false",
				"is_featured": "false",
				"keywords":    "statistics, machine learning, comparison",
				"sections": map[string]interface{}{
					"introduction": map[string]interface{}{
						"length": "450",
						"topics": []string{"history", "overview"},
					},
					"comparison": map[string]interface{}{
						"length": "2500",
						"topics": []string{"methods", "applications", "limitations"},
						"tables": "8",
					},
				},
			},
		},
	}

	// 存储向量和元数据
	err = store.Store(ctx, vectors)
	if err != nil {
		t.Fatalf("Failed to store vectors: %v", err)
	}

	// 准备查询向量
	queryVector := memory.Vector{
		Values: []float32{1.0, 1.0, 1.0}, // 通用查询向量
	}

	// 测试用例集合
	testCases := []struct {
		name          string
		filter        map[string]interface{}
		expectedCount int
		expectedIDs   []string
	}{
		{
			name: "基本精确匹配",
			filter: map[string]interface{}{
				"author": "John Smith",
			},
			expectedCount: 1,
			expectedIDs:   []string{"article1"},
		},
		{
			name: "比较运算符 - 大于",
			filter: map[string]interface{}{
				"rating": map[string]interface{}{
					"$gt": "4.0",
				},
			},
			expectedCount: 3,
			expectedIDs:   []string{"article1", "article2", "article3"},
		},
		{
			name: "比较运算符 - 小于等于",
			filter: map[string]interface{}{
				"rating": map[string]interface{}{
					"$lte": "4.0",
				},
			},
			expectedCount: 2,
			expectedIDs:   []string{"article3", "article4"},
		},
		{
			name: "正则表达式匹配",
			filter: map[string]interface{}{
				"title": map[string]interface{}{
					"$regex": "^Machine",
				},
			},
			expectedCount: 2,
			expectedIDs:   []string{"article1", "article4"},
		},
		{
			name: "嵌套字段过滤",
			filter: map[string]interface{}{
				"sections.models.length": "3000",
			},
			expectedCount: 1,
			expectedIDs:   []string{"article2"},
		},
		{
			name: "嵌套字段比较运算符",
			filter: map[string]interface{}{
				"sections.algorithms.code_examples": map[string]interface{}{
					"$gt": "15",
				},
			},
			expectedCount: 1,
			expectedIDs:   []string{"article1"},
		},
		{
			name: "多条件过滤 - AND",
			filter: map[string]interface{}{
				"published": map[string]interface{}{
					"$gte": "2022",
				},
				"is_featured": "true",
			},
			expectedCount: 2,
			expectedIDs:   []string{"article1", "article2"},
		},
		{
			name: "关键词搜索",
			filter: map[string]interface{}{
				"keywords": map[string]interface{}{
					"$regex": "neural networks",
				},
			},
			expectedCount: 1,
			expectedIDs:   []string{"article2"},
		},
	}

	// 执行测试用例
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := memory.VectorSearchOptions{
				Limit:  10,
				Filter: tc.filter,
			}

			results, err := store.Search(ctx, queryVector, opts)
			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}

			if len(results) != tc.expectedCount {
				t.Errorf("Expected %d results, got %d", tc.expectedCount, len(results))
			}

			// 验证找到的ID是否符合期望
			foundIDs := make(map[string]bool)
			for _, result := range results {
				foundIDs[result.Vector.ID] = true
			}

			for _, expectedID := range tc.expectedIDs {
				if !foundIDs[expectedID] {
					t.Errorf("Expected to find ID %s, but it was missing", expectedID)
				}
			}
		})
	}
}
