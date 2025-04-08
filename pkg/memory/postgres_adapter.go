package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq" // PostgreSQL 驱动
)

// PostgresMemoryProvider 实现了基于PostgreSQL的内存存储
type PostgresMemoryProvider struct {
	db            *sql.DB
	embedProvider EmbeddingProvider
}

// PostgresOptions PostgreSQL配置选项
type PostgresOptions struct {
	ConnectionString   string
	TablePrefix        string
	EmbeddingProvider  EmbeddingProvider
	EnableVectorSearch bool
}

// NewPostgresMemoryProvider 创建一个新的PostgreSQL内存提供者
func NewPostgresMemoryProvider(opts *PostgresOptions) (*PostgresMemoryProvider, error) {
	if opts == nil {
		return nil, errors.New("options cannot be nil")
	}

	if opts.ConnectionString == "" {
		return nil, errors.New("connection string is required")
	}

	// 连接数据库
	db, err := sql.Open("postgres", opts.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// 测试连接
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	// 创建提供者
	provider := &PostgresMemoryProvider{
		db:            db,
		embedProvider: opts.EmbeddingProvider,
	}

	// 初始化表
	if err := provider.initTables(opts.TablePrefix, opts.EnableVectorSearch); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	return provider, nil
}

// 初始化数据库表
func (p *PostgresMemoryProvider) initTables(prefix string, enableVectorSearch bool) error {
	// 使用前缀
	prefix = strings.TrimSuffix(prefix, "_")
	if prefix != "" {
		prefix += "_"
	}

	// 创建线程表
	threadTable := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %sthreads (
			id TEXT PRIMARY KEY,
			metadata JSONB,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL
		);
	`, prefix)

	if _, err := p.db.Exec(threadTable); err != nil {
		return fmt.Errorf("failed to create threads table: %w", err)
	}

	// 创建消息表
	messageTable := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %smessages (
			id TEXT PRIMARY KEY,
			thread_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			metadata JSONB,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL,
			FOREIGN KEY (thread_id) REFERENCES %sthreads(id) ON DELETE CASCADE
		);
	`, prefix, prefix)

	if _, err := p.db.Exec(messageTable); err != nil {
		return fmt.Errorf("failed to create messages table: %w", err)
	}

	// 创建消息索引
	messageIndex := fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS %smessages_thread_id_idx ON %smessages(thread_id);
	`, prefix, prefix)

	if _, err := p.db.Exec(messageIndex); err != nil {
		return fmt.Errorf("failed to create message index: %w", err)
	}

	// 如果启用向量搜索，创建向量表
	if enableVectorSearch {
		// 检查pgvector扩展是否已安装
		var exists bool
		err := p.db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'vector')").Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check pgvector extension: %w", err)
		}

		if !exists {
			// 创建pgvector扩展
			if _, err := p.db.Exec("CREATE EXTENSION IF NOT EXISTS vector;"); err != nil {
				return fmt.Errorf("failed to create pgvector extension: %w", err)
			}
		}

		// 创建向量表
		vectorTable := fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %svectors (
				id TEXT PRIMARY KEY,
				message_id TEXT NOT NULL,
				thread_id TEXT NOT NULL,
				content TEXT NOT NULL,
				metadata JSONB,
				embedding vector(1536),
				created_at TIMESTAMP WITH TIME ZONE NOT NULL,
				FOREIGN KEY (message_id) REFERENCES %smessages(id) ON DELETE CASCADE,
				FOREIGN KEY (thread_id) REFERENCES %sthreads(id) ON DELETE CASCADE
			);
		`, prefix, prefix, prefix)

		if _, err := p.db.Exec(vectorTable); err != nil {
			return fmt.Errorf("failed to create vectors table: %w", err)
		}

		// 创建向量索引
		vectorIndex := fmt.Sprintf(`
			CREATE INDEX IF NOT EXISTS %svectors_thread_id_idx ON %svectors(thread_id);
		`, prefix, prefix)

		if _, err := p.db.Exec(vectorIndex); err != nil {
			return fmt.Errorf("failed to create vector index: %w", err)
		}

		// 创建向量搜索索引
		vectorSearchIndex := fmt.Sprintf(`
			CREATE INDEX IF NOT EXISTS %svectors_embedding_idx ON %svectors USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
		`, prefix, prefix)

		if _, err := p.db.Exec(vectorSearchIndex); err != nil {
			return fmt.Errorf("failed to create vector search index: %w", err)
		}
	}

	return nil
}

// Close 关闭数据库连接
func (p *PostgresMemoryProvider) Close() error {
	return p.db.Close()
}

// CreateThread 创建一个新的线程
func (p *PostgresMemoryProvider) CreateThread(ctx context.Context, metadata map[string]interface{}) (*Thread, error) {
	threadID := generateID()
	nowUTC := time.Now().UTC()

	// 序列化元数据
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// 插入线程
	_, err = p.db.ExecContext(
		ctx,
		"INSERT INTO threads (id, metadata, created_at) VALUES ($1, $2, $3)",
		threadID, metadataJSON, nowUTC,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert thread: %w", err)
	}

	return &Thread{
		ID:        threadID,
		Metadata:  metadata,
		CreatedAt: nowUTC,
	}, nil
}

// GetThread 获取一个线程
func (p *PostgresMemoryProvider) GetThread(ctx context.Context, threadID string) (*Thread, error) {
	var (
		id          string
		metadataRaw []byte
		createdAt   time.Time
	)

	err := p.db.QueryRowContext(
		ctx,
		"SELECT id, metadata, created_at FROM threads WHERE id = $1",
		threadID,
	).Scan(&id, &metadataRaw, &createdAt)

	if err == sql.ErrNoRows {
		return nil, errors.New("thread not found")
	} else if err != nil {
		return nil, fmt.Errorf("failed to get thread: %w", err)
	}

	// 解析元数据
	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataRaw, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &Thread{
		ID:        id,
		Metadata:  metadata,
		CreatedAt: createdAt,
	}, nil
}

// UpdateThread 更新线程元数据
func (p *PostgresMemoryProvider) UpdateThread(ctx context.Context, threadID string, metadata map[string]interface{}) (*Thread, error) {
	// 检查线程是否存在
	thread, err := p.GetThread(ctx, threadID)
	if err != nil {
		return nil, err
	}

	// 序列化元数据
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// 更新线程
	_, err = p.db.ExecContext(
		ctx,
		"UPDATE threads SET metadata = $1 WHERE id = $2",
		metadataJSON, threadID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update thread: %w", err)
	}

	thread.Metadata = metadata
	return thread, nil
}

// DeleteThread 删除一个线程及其消息
func (p *PostgresMemoryProvider) DeleteThread(ctx context.Context, threadID string) error {
	// 由于外键约束，删除线程时会自动删除相关的消息和向量
	result, err := p.db.ExecContext(
		ctx,
		"DELETE FROM threads WHERE id = $1",
		threadID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete thread: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return errors.New("thread not found")
	}

	return nil
}

// ListThreads 列出所有线程
func (p *PostgresMemoryProvider) ListThreads(ctx context.Context, limit int, offset int) ([]*Thread, error) {
	rows, err := p.db.QueryContext(
		ctx,
		"SELECT id, metadata, created_at FROM threads ORDER BY created_at DESC LIMIT $1 OFFSET $2",
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query threads: %w", err)
	}
	defer rows.Close()

	threads := []*Thread{}
	for rows.Next() {
		var (
			id          string
			metadataRaw []byte
			createdAt   time.Time
		)

		if err := rows.Scan(&id, &metadataRaw, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan thread: %w", err)
		}

		// 解析元数据
		var metadata map[string]interface{}
		if err := json.Unmarshal(metadataRaw, &metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		threads = append(threads, &Thread{
			ID:        id,
			Metadata:  metadata,
			CreatedAt: createdAt,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating threads: %w", err)
	}

	return threads, nil
}

// AddMessage 添加一条消息到线程
func (p *PostgresMemoryProvider) AddMessage(ctx context.Context, threadID string, role string, content string, metadata map[string]interface{}) (*Message, error) {
	// 检查线程是否存在
	_, err := p.GetThread(ctx, threadID)
	if err != nil {
		return nil, err
	}

	messageID := generateID()
	nowUTC := time.Now().UTC()

	// 序列化元数据
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// 开始事务
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// 插入消息
	_, err = tx.ExecContext(
		ctx,
		"INSERT INTO messages (id, thread_id, role, content, metadata, created_at) VALUES ($1, $2, $3, $4, $5, $6)",
		messageID, threadID, role, content, metadataJSON, nowUTC,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert message: %w", err)
	}

	// 如果存在嵌入提供者，创建向量嵌入
	if p.embedProvider != nil {
		// 获取嵌入向量
		embedding, err := p.embedProvider.GetEmbedding(ctx, content)
		if err == nil { // 如果获取嵌入向量成功
			// 检查是否存在向量表
			var exists bool
			err := tx.QueryRowContext(ctx, `
				SELECT EXISTS (
					SELECT FROM information_schema.tables 
					WHERE table_name = 'vectors'
				)
			`).Scan(&exists)

			if err == nil && exists {
				// 构建向量元数据
				vectorMetadata := map[string]interface{}{
					"thread_id": threadID,
					"role":      role,
				}
				if metadata != nil {
					for k, v := range metadata {
						vectorMetadata[k] = v
					}
				}

				// 序列化向量元数据
				vectorMetadataJSON, _ := json.Marshal(vectorMetadata)

				// 插入向量
				_, err = tx.ExecContext(
					ctx,
					"INSERT INTO vectors (id, message_id, thread_id, content, metadata, embedding, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
					generateID(), messageID, threadID, content, vectorMetadataJSON, formatPgVector(embedding), nowUTC,
				)
				if err != nil {
					// 记录错误但继续执行
					fmt.Printf("Failed to insert vector: %v\n", err)
				}
			}
		}
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &Message{
		ID:        messageID,
		Role:      role,
		Content:   content,
		Metadata:  metadata,
		CreatedAt: nowUTC,
	}, nil
}

// GetMessages 获取线程中的消息
func (p *PostgresMemoryProvider) GetMessages(ctx context.Context, threadID string, limit int, offset int) ([]*Message, error) {
	// 检查线程是否存在
	_, err := p.GetThread(ctx, threadID)
	if err != nil {
		return nil, err
	}

	rows, err := p.db.QueryContext(
		ctx,
		"SELECT id, role, content, metadata, created_at FROM messages WHERE thread_id = $1 ORDER BY created_at ASC LIMIT $2 OFFSET $3",
		threadID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	messages := []*Message{}
	for rows.Next() {
		var (
			id          string
			role        string
			content     string
			metadataRaw []byte
			createdAt   time.Time
		)

		if err := rows.Scan(&id, &role, &content, &metadataRaw, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		// 解析元数据
		var metadata map[string]interface{}
		if err := json.Unmarshal(metadataRaw, &metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		messages = append(messages, &Message{
			ID:        id,
			Role:      role,
			Content:   content,
			Metadata:  metadata,
			CreatedAt: createdAt,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating messages: %w", err)
	}

	return messages, nil
}

// DeleteMessages 从线程中删除消息
func (p *PostgresMemoryProvider) DeleteMessages(ctx context.Context, threadID string, messageIDs []string) error {
	// 检查线程是否存在
	_, err := p.GetThread(ctx, threadID)
	if err != nil {
		return err
	}

	if len(messageIDs) == 0 {
		return nil
	}

	// 构建占位符
	placeholders := make([]string, len(messageIDs))
	args := make([]interface{}, len(messageIDs)+1)
	args[0] = threadID

	for i, id := range messageIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args[i+1] = id
	}

	// 删除消息
	query := fmt.Sprintf(
		"DELETE FROM messages WHERE thread_id = $1 AND id IN (%s)",
		strings.Join(placeholders, ", "),
	)

	_, err = p.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete messages: %w", err)
	}

	return nil
}

// SearchVectorsByText 使用文本进行向量搜索
func (p *PostgresMemoryProvider) SearchVectorsByText(ctx context.Context, text string, threadID string, limit int) ([]*Message, error) {
	if p.embedProvider == nil {
		return nil, errors.New("embedding provider is required for vector search")
	}

	// 获取嵌入向量
	embedding, err := p.embedProvider.GetEmbedding(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("failed to get embedding: %w", err)
	}

	return p.SearchVectors(ctx, embedding, threadID, limit)
}

// SearchVectors 使用向量进行搜索
func (p *PostgresMemoryProvider) SearchVectors(ctx context.Context, embedding Embedding, threadID string, limit int) ([]*Message, error) {
	// 检查是否存在向量表
	var exists bool
	err := p.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'vectors'
		)
	`).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("failed to check vectors table: %w", err)
	}

	if !exists {
		return nil, errors.New("vector search is not enabled")
	}

	// 构建查询
	query := `
		SELECT m.id, m.role, m.content, m.metadata, m.created_at, v.embedding <=> $1 AS distance
		FROM vectors v
		JOIN messages m ON v.message_id = m.id
		WHERE 1=1
	`
	args := []interface{}{formatPgVector(embedding)}
	argCount := 2

	// 添加线程过滤条件
	if threadID != "" {
		query += fmt.Sprintf(" AND v.thread_id = $%d", argCount)
		args = append(args, threadID)
		argCount++
	}

	// 添加排序和限制
	query += " ORDER BY distance LIMIT $" + fmt.Sprint(argCount)
	args = append(args, limit)

	// 执行查询
	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query vectors: %w", err)
	}
	defer rows.Close()

	messages := []*Message{}
	for rows.Next() {
		var (
			id          string
			role        string
			content     string
			metadataRaw []byte
			createdAt   time.Time
			distance    float64
		)

		if err := rows.Scan(&id, &role, &content, &metadataRaw, &createdAt, &distance); err != nil {
			return nil, fmt.Errorf("failed to scan vector result: %w", err)
		}

		// 解析元数据
		var metadata map[string]interface{}
		if err := json.Unmarshal(metadataRaw, &metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		// 添加距离/相似度分数到元数据
		if metadata == nil {
			metadata = make(map[string]interface{})
		}
		metadata["similarity_score"] = 1.0 - distance

		messages = append(messages, &Message{
			ID:        id,
			Role:      role,
			Content:   content,
			Metadata:  metadata,
			CreatedAt: createdAt,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating vector results: %w", err)
	}

	return messages, nil
}

// 将嵌入向量格式化为PostgreSQL向量格式
func formatPgVector(embedding Embedding) string {
	elements := make([]string, len(embedding))
	for i, v := range embedding {
		elements[i] = fmt.Sprintf("%f", v)
	}
	return "[" + strings.Join(elements, ",") + "]"
}
