package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ReportFormat defines the format of the evaluation report
type ReportFormat string

const (
	// JSONFormat represents JSON report format
	JSONFormat ReportFormat = "json"
	// MarkdownFormat represents Markdown report format (for human-readable reports)
	MarkdownFormat ReportFormat = "md"
	// CSVFormat represents CSV report format (for tabular data)
	CSVFormat ReportFormat = "csv"
)

// ReportConfig defines configuration for report generation
type ReportConfig struct {
	Format       ReportFormat         `json:"format"`
	OutputPath   string               `json:"output_path"`
	IncludeRaw   bool                 `json:"include_raw"`   // Include raw data in report
	CustomFields map[string]string    `json:"custom_fields"` // Custom fields to include in report
	MetricsOrder []string             `json:"metrics_order"` // Order of metrics in report
	Filters      map[string][]float64 `json:"filters"`       // Filters for metrics (min/max values)
}

// GenerateReport generates a report from an EvalResult
func GenerateReport(result *EvalResult, config *ReportConfig) error {
	switch config.Format {
	case JSONFormat:
		return generateJSONReport(result, nil, config)
	case MarkdownFormat:
		return generateMarkdownReport(result, nil, config)
	case CSVFormat:
		return generateCSVReport(result, nil, config)
	default:
		return fmt.Errorf("unsupported report format: %s", config.Format)
	}
}

// GenerateBatchReport generates a report from a BatchEvalResult
func GenerateBatchReport(result *BatchEvalResult, config *ReportConfig) error {
	switch config.Format {
	case JSONFormat:
		return generateJSONReport(nil, result, config)
	case MarkdownFormat:
		return generateMarkdownReport(nil, result, config)
	case CSVFormat:
		return generateCSVReport(nil, result, config)
	default:
		return fmt.Errorf("unsupported report format: %s", config.Format)
	}
}

// generateJSONReport generates a JSON report
func generateJSONReport(result *EvalResult, batchResult *BatchEvalResult, config *ReportConfig) error {
	var data interface{}
	if result != nil {
		data = result
	} else {
		data = batchResult
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(config.OutputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Marshal data to JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data to JSON: %w", err)
	}

	// Write JSON to file
	if err := os.WriteFile(config.OutputPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write JSON report: %w", err)
	}

	return nil
}

// generateMarkdownReport generates a Markdown report
func generateMarkdownReport(result *EvalResult, batchResult *BatchEvalResult, config *ReportConfig) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(config.OutputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	file, err := os.Create(config.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to create report file: %w", err)
	}
	defer file.Close()

	// Generate markdown content
	if result != nil {
		// Single evaluation report
		fmt.Fprintf(file, "# Evaluation Report\n\n")
		fmt.Fprintf(file, "## Overview\n\n")
		fmt.Fprintf(file, "- **Name**: %s\n", result.Name)
		fmt.Fprintf(file, "- **ID**: %s\n", result.ID)
		fmt.Fprintf(file, "- **Description**: %s\n", result.Description)
		fmt.Fprintf(file, "- **Date**: %s\n", result.StartTime.Format(time.RFC1123))
		fmt.Fprintf(file, "- **Duration**: %s\n", result.Duration.String())
		fmt.Fprintf(file, "- **Score**: %.2f\n", result.Score)
		fmt.Fprintf(file, "- **Success**: %t\n", result.Success)

		if result.Error != "" {
			fmt.Fprintf(file, "- **Error**: %s\n", result.Error)
		}

		fmt.Fprintf(file, "\n## Metrics\n\n")
		fmt.Fprintf(file, "| Metric | Value |\n")
		fmt.Fprintf(file, "|--------|-------|\n")

		// Use metrics order if provided
		if len(config.MetricsOrder) > 0 {
			for _, metricName := range config.MetricsOrder {
				if value, ok := result.Metrics[metricName]; ok {
					fmt.Fprintf(file, "| %s | %.4f |\n", metricName, value)
				}
			}
		}

		// Add remaining metrics not in the order
		for metricName, value := range result.Metrics {
			// Skip if already included in ordered metrics
			if contains(config.MetricsOrder, metricName) {
				continue
			}
			fmt.Fprintf(file, "| %s | %.4f |\n", metricName, value)
		}

		// Include metadata if available
		if len(result.Metadata) > 0 {
			fmt.Fprintf(file, "\n## Metadata\n\n")
			for key, value := range result.Metadata {
				fmt.Fprintf(file, "- **%s**: %v\n", key, value)
			}
		}
	} else if batchResult != nil {
		// Batch evaluation report
		fmt.Fprintf(file, "# Batch Evaluation Report\n\n")
		fmt.Fprintf(file, "## Overview\n\n")
		fmt.Fprintf(file, "- **Agent ID**: %s\n", batchResult.AgentID)
		fmt.Fprintf(file, "- **Configuration**: %s\n", batchResult.EvalConfig.Name)
		fmt.Fprintf(file, "- **Date**: %s\n", batchResult.StartTime.Format(time.RFC1123))
		fmt.Fprintf(file, "- **Total Duration**: %s\n", batchResult.TotalDuration.String())

		fmt.Fprintf(file, "\n## Summary\n\n")
		fmt.Fprintf(file, "- **Total Cases**: %d\n", batchResult.Summary.TotalCases)
		fmt.Fprintf(file, "- **Successful**: %d\n", batchResult.Summary.Successful)
		fmt.Fprintf(file, "- **Failed**: %d\n", batchResult.Summary.Failed)
		fmt.Fprintf(file, "- **Success Rate**: %.2f%%\n", float64(batchResult.Summary.Successful)/float64(batchResult.Summary.TotalCases)*100)
		fmt.Fprintf(file, "- **Average Score**: %.4f\n", batchResult.Summary.AverageScore)
		fmt.Fprintf(file, "- **Average Duration**: %s\n", batchResult.Summary.AverageDuration.String())

		fmt.Fprintf(file, "\n## Individual Results\n\n")
		fmt.Fprintf(file, "| ID | Score | Success | Duration |\n")
		fmt.Fprintf(file, "|-----|-------|---------|----------|\n")

		for _, result := range batchResult.Results {
			fmt.Fprintf(file, "| %s | %.4f | %t | %s |\n", result.ID, result.Score, result.Success, result.Duration.String())
		}

		// Include detailed metrics if requested
		if config.IncludeRaw {
			fmt.Fprintf(file, "\n## Detailed Metrics\n\n")

			for i, result := range batchResult.Results {
				fmt.Fprintf(file, "### Case %d: %s\n\n", i+1, result.ID)
				fmt.Fprintf(file, "- **Score**: %.4f\n", result.Score)
				fmt.Fprintf(file, "- **Success**: %t\n", result.Success)

				fmt.Fprintf(file, "\n#### Metrics\n\n")
				fmt.Fprintf(file, "| Metric | Value |\n")
				fmt.Fprintf(file, "|--------|-------|\n")

				for metricName, value := range result.Metrics {
					fmt.Fprintf(file, "| %s | %.4f |\n", metricName, value)
				}

				fmt.Fprintf(file, "\n")
			}
		}
	}

	return nil
}

// generateCSVReport generates a CSV report
func generateCSVReport(result *EvalResult, batchResult *BatchEvalResult, config *ReportConfig) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(config.OutputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	file, err := os.Create(config.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to create report file: %w", err)
	}
	defer file.Close()

	if result != nil {
		// Single evaluation CSV
		fmt.Fprintf(file, "ID,Name,Description,StartTime,EndTime,Duration,Success,Score")

		// Add metrics as columns
		for metricName := range result.Metrics {
			fmt.Fprintf(file, ",%s", metricName)
		}
		fmt.Fprintf(file, "\n")

		// Add result row
		fmt.Fprintf(file, "%s,%s,%s,%s,%s,%s,%t,%.4f",
			result.ID,
			escapeCsvField(result.Name),
			escapeCsvField(result.Description),
			result.StartTime.Format(time.RFC3339),
			result.EndTime.Format(time.RFC3339),
			result.Duration.String(),
			result.Success,
			result.Score)

		// Add metric values
		for _, value := range result.Metrics {
			fmt.Fprintf(file, ",%.4f", value)
		}
		fmt.Fprintf(file, "\n")
	} else if batchResult != nil {
		// Batch evaluation CSV
		// First, find all unique metrics across all results
		allMetrics := make(map[string]bool)
		for _, res := range batchResult.Results {
			for metricName := range res.Metrics {
				allMetrics[metricName] = true
			}
		}

		// Create header
		fmt.Fprintf(file, "ID,Name,Description,StartTime,EndTime,Duration,Success,Score")
		for metricName := range allMetrics {
			fmt.Fprintf(file, ",%s", metricName)
		}
		fmt.Fprintf(file, "\n")

		// Add rows for each result
		for _, res := range batchResult.Results {
			fmt.Fprintf(file, "%s,%s,%s,%s,%s,%s,%t,%.4f",
				res.ID,
				escapeCsvField(res.Name),
				escapeCsvField(res.Description),
				res.StartTime.Format(time.RFC3339),
				res.EndTime.Format(time.RFC3339),
				res.Duration.String(),
				res.Success,
				res.Score)

			// Add metric values (or empty if not present)
			for metricName := range allMetrics {
				if value, ok := res.Metrics[metricName]; ok {
					fmt.Fprintf(file, ",%.4f", value)
				} else {
					fmt.Fprintf(file, ",")
				}
			}
			fmt.Fprintf(file, "\n")
		}

		// Add summary row
		fmt.Fprintf(file, "SUMMARY,Batch Summary,%s,%s,%s,%s,%.2f%%,%.4f\n",
			batchResult.EvalConfig.Description,
			batchResult.StartTime.Format(time.RFC3339),
			batchResult.EndTime.Format(time.RFC3339),
			batchResult.TotalDuration.String(),
			float64(batchResult.Summary.Successful)/float64(batchResult.Summary.TotalCases)*100,
			batchResult.Summary.AverageScore)
	}

	return nil
}

// Helper function to check if a string is in a slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Helper function to escape CSV fields
func escapeCsvField(field string) string {
	if strings.Contains(field, ",") || strings.Contains(field, "\"") || strings.Contains(field, "\n") {
		return fmt.Sprintf("\"%s\"", strings.ReplaceAll(field, "\"", "\"\""))
	}
	return field
}
