# Graph RAG Example

This example demonstrates how to use the Graph RAG (Graph-based Retrieval Augmented Generation) tool with Gostra's actor architecture.

## Overview

Graph RAG extends traditional RAG by building a knowledge graph from document chunks and using graph algorithms like PageRank to find the most relevant information. The key components involved are:

1. **Document Processing**: Converting documents into chunks that can be embedded.
2. **Vector Storage**: Storing document embeddings in a PostgreSQL database with pgvector.
3. **Knowledge Graph Construction**: Building a graph where nodes are document chunks and edges represent relationships.
4. **Graph Traversal**: Using PageRank to identify the most important nodes in the graph.
5. **Actor Integration**: Integrating with Gostra's actor system for asynchronous processing.

## Prerequisites

- Go 1.18 or higher
- PostgreSQL with pgvector extension installed
- OpenAI API key for embeddings and language model

## Environment Setup

```bash
# OpenAI API key
export OPENAI_API_KEY=your_openai_api_key

# PostgreSQL connection string
export POSTGRES_CONNECTION_STRING=postgresql://username:password@localhost:5432/yourdatabase
```

## Key Components

### 1. Knowledge Graph Structure

The knowledge graph consists of:

- **Nodes**: Represent document chunks with their content and metadata
- **Edges**: Connections between nodes based on their semantic similarity
- **Query Node**: A special node representing the user's query

### 2. PageRank Algorithm

The PageRank algorithm is used to score nodes based on:

- Their direct relationship to the query
- Their relationships with other nodes
- The importance of connected nodes

### 3. Actor Integration

The Graph RAG tool is integrated with Gostra's actor system:

- **Graph RAG Tool**: Implements the Tool interface for graph construction and traversal
- **Graph RAG Agent**: An agent that uses the Graph RAG tool to answer questions
- **Actor Message Passing**: Asynchronous communication between components

## Running the Example

```bash
cd gostra
go run examples/graph_rag_example/main.go
```

## Example Queries

The example includes sample queries that demonstrate different aspects of Graph RAG:

1. "What are the main historical stages of development in the case study?"
2. "How did the railroad impact development? Find relationships through the knowledge graph."
3. "How did the city respond to industrial decline, and what strategies were used?"

## Output Format

The output follows a structured format to highlight the benefits of graph-based retrieval:

```
===== Query: How did the railroad impact development? =====

1. DIRECT FACTS:
   - The railroad arrived in 1870, connecting Riverdale to major cities
   - The arrival of the railroad greatly boosted the local economy
   - By the end of the 19th century, after the railroad arrived, Riverdale had become an important industrial center

2. CONNECTIONS:
   - The railroad (1870) preceded the coal mine operation (1885), suggesting the railroad enabled energy resource development
   - The transportation connectivity from the railroad led to population growth from 300 people in 1840 to 3,000 by the end of the century
   - The economic growth from the railroad in the industrial period created the foundation for later industrial prosperity (1900-1950)

3. CONCLUSION:
   The railroad served as a critical inflection point in Riverdale's development, transforming it from a small settlement into an industrial center by enabling transportation, resource development, and economic growth.
```

## Extending the Example

You can extend this example by:

1. Adding more document sources
2. Implementing different graph construction methods
3. Using different graph algorithms for traversal
4. Adding visualization for the knowledge graph

## Additional Resources

- [Graph RAG documentation](https://github.com/louloulin/gostra/docs/graph_rag.md)
- [Knowledge graph visualization tools](https://github.com/louloulin/gostra/tools/graph_viz/README.md)
- [Advanced metadata filtering with Graph RAG](https://github.com/louloulin/gostra/examples/advanced_filtering/README.md) 