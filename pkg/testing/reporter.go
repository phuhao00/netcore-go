// Package testing JSON报告器实现
// Author: NetCore-Go Team
// Created: 2024

package testing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// JSONReporter JSON格式报告器
type JSONReporter struct {
	outputPath string
	report     *TestReport
}

// ConsoleReporter 控制台报告器
type ConsoleReporter struct {
	verbose bool
	colors  bool
}

// XMLReporter XML格式报告器
type XMLReporter struct {
	outputPath string
	report     *TestReport
}

// HTMLReporter HTML格式报告器
type HTMLReporter struct {
	outputPath string
	report     *TestReport
	template   string
}

// TestReport 测试报告
type TestReport struct {
	Name        string        `json:"name"`
	StartTime   time.Time     `json:"start_time"`
	EndTime     time.Time     `json:"end_time"`
	Duration    time.Duration `json:"duration"`
	Stats       *TestStats    `json:"stats"`
	Suites      []*SuiteReport `json:"suites"`
	Environment *Environment  `json:"environment"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// SuiteReport 套件报告
type SuiteReport struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Type        TestType      `json:"type"`
	Tags        []string      `json:"tags"`
	StartTime   time.Time     `json:"start_time"`
	EndTime     time.Time     `json:"end_time"`
	Duration    time.Duration `json:"duration"`
	Tests       []*TestCase   `json:"tests"`
	Stats       *SuiteStats   `json:"stats"`
}

// SuiteStats 套件统计
type SuiteStats struct {
	TotalTests   int `json:"total_tests"`
	PassedTests  int `json:"passed_tests"`
	FailedTests  int `json:"failed_tests"`
	SkippedTests int `json:"skipped_tests"`
	TimeoutTests int `json:"timeout_tests"`
}

// Environment 环境信息
type Environment struct {
	OS           string            `json:"os"`
	Arch         string            `json:"arch"`
	GoVersion    string            `json:"go_version"`
	Hostname     string            `json:"hostname"`
	WorkingDir   string            `json:"working_dir"`
	Environment  map[string]string `json:"environment"`
	Timezone     string            `json:"timezone"`
}

// NewJSONReporter 创建JSON报告器
func NewJSONReporter(outputPath string) *JSONReporter {
	return &JSONReporter{
		outputPath: outputPath,
		report: &TestReport{
			Suites:      make([]*SuiteReport, 0),
			Environment: collectEnvironmentInfo(),
			Metadata:    make(map[string]interface{}),
		},
	}
}

// NewConsoleReporter 创建控制台报告器
func NewConsoleReporter(verbose, colors bool) *ConsoleReporter {
	return &ConsoleReporter{
		verbose: verbose,
		colors:  colors,
	}
}

// NewXMLReporter 创建XML报告器
func NewXMLReporter(outputPath string) *XMLReporter {
	return &XMLReporter{
		outputPath: outputPath,
		report: &TestReport{
			Suites:      make([]*SuiteReport, 0),
			Environment: collectEnvironmentInfo(),
			Metadata:    make(map[string]interface{}),
		},
	}
}

// NewHTMLReporter 创建HTML报告器
func NewHTMLReporter(outputPath string) *HTMLReporter {
	return &HTMLReporter{
		outputPath: outputPath,
		report: &TestReport{
			Suites:      make([]*SuiteReport, 0),
			Environment: collectEnvironmentInfo(),
			Metadata:    make(map[string]interface{}),
		},
		template: defaultHTMLTemplate,
	}
}

// JSONReporter 实现

// ReportStart 开始报告
func (jr *JSONReporter) ReportStart(stats *TestStats) {
	jr.report.Name = "NetCore-Go Test Report"
	jr.report.StartTime = time.Now()
	jr.report.Stats = stats
}

// ReportSuite 报告套件
func (jr *JSONReporter) ReportSuite(suite *TestSuite, stats *TestStats) {
	suiteReport := &SuiteReport{
		Name:        suite.Name,
		Description: suite.Description,
		Type:        suite.Type,
		Tags:        suite.Tags,
		StartTime:   time.Now(),
		Tests:       suite.Tests,
		Stats:       calculateSuiteStats(suite.Tests),
	}

	// 计算套件持续时间
	if len(suite.Tests) > 0 {
		var minStart, maxEnd time.Time
		for i, test := range suite.Tests {
			if i == 0 || test.StartTime.Before(minStart) {
				minStart = test.StartTime
			}
			if i == 0 || test.EndTime.After(maxEnd) {
				maxEnd = test.EndTime
			}
		}
		suiteReport.StartTime = minStart
		suiteReport.EndTime = maxEnd
		suiteReport.Duration = maxEnd.Sub(minStart)
	}

	jr.report.Suites = append(jr.report.Suites, suiteReport)
}

// ReportTest 报告测试
func (jr *JSONReporter) ReportTest(test *TestCase) {
	// 测试结果已经在TestCase中，无需额外处理
}

// ReportEnd 结束报告
func (jr *JSONReporter) ReportEnd(stats *TestStats) {
	jr.report.EndTime = time.Now()
	jr.report.Duration = jr.report.EndTime.Sub(jr.report.StartTime)
	jr.report.Stats = stats
}

// GenerateReport 生成报告
func (jr *JSONReporter) GenerateReport() ([]byte, error) {
	data, err := json.MarshalIndent(jr.report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal report: %w", err)
	}

	// 写入文件
	if jr.outputPath != "" {
		dir := filepath.Dir(jr.outputPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}

		if err := os.WriteFile(jr.outputPath, data, 0644); err != nil {
			return nil, fmt.Errorf("failed to write report file: %w", err)
		}
	}

	return data, nil
}

// ConsoleReporter 实现

// ReportStart 开始报告
func (cr *ConsoleReporter) ReportStart(stats *TestStats) {
	fmt.Println(cr.colorize("=== NetCore-Go Test Framework ===", "cyan"))
	fmt.Printf("Started at: %s\n", time.Now().Format(time.RFC3339))
	fmt.Println()
}

// ReportSuite 报告套件
func (cr *ConsoleReporter) ReportSuite(suite *TestSuite, stats *TestStats) {
	suiteStats := calculateSuiteStats(suite.Tests)
	fmt.Printf("Suite: %s\n", cr.colorize(suite.Name, "blue"))
	if suite.Description != "" {
		fmt.Printf("  Description: %s\n", suite.Description)
	}
	fmt.Printf("  Type: %s\n", suite.Type)
	if len(suite.Tags) > 0 {
		fmt.Printf("  Tags: %s\n", strings.Join(suite.Tags, ", "))
	}
	fmt.Printf("  Tests: %d passed, %d failed, %d skipped\n",
		suiteStats.PassedTests, suiteStats.FailedTests, suiteStats.SkippedTests)
	fmt.Println()
}

// ReportTest 报告测试
func (cr *ConsoleReporter) ReportTest(test *TestCase) {
	var status string
	switch test.Status {
	case TestStatusPassed:
		status = cr.colorize("PASS", "green")
	case TestStatusFailed:
		status = cr.colorize("FAIL", "red")
	case TestStatusSkipped:
		status = cr.colorize("SKIP", "yellow")
	case TestStatusTimeout:
		status = cr.colorize("TIMEOUT", "red")
	default:
		status = string(test.Status)
	}

	fmt.Printf("  %s %s (%s)", status, test.Name, test.Duration)
	if test.Retries > 0 {
		fmt.Printf(" [retries: %d]", test.Retries)
	}
	fmt.Println()

	if cr.verbose && test.Error != "" {
		fmt.Printf("    Error: %s\n", cr.colorize(test.Error, "red"))
	}
}

// ReportEnd 结束报告
func (cr *ConsoleReporter) ReportEnd(stats *TestStats) {
	fmt.Println()
	fmt.Println(cr.colorize("=== Test Summary ===", "cyan"))
	fmt.Printf("Total Tests: %d\n", stats.TotalTests)
	fmt.Printf("Passed: %s\n", cr.colorize(fmt.Sprintf("%d", stats.PassedTests), "green"))
	fmt.Printf("Failed: %s\n", cr.colorize(fmt.Sprintf("%d", stats.FailedTests), "red"))
	fmt.Printf("Skipped: %s\n", cr.colorize(fmt.Sprintf("%d", stats.SkippedTests), "yellow"))
	fmt.Printf("Timeout: %s\n", cr.colorize(fmt.Sprintf("%d", stats.TimeoutTests), "red"))
	fmt.Printf("Success Rate: %s\n", cr.colorize(fmt.Sprintf("%.2f%%", stats.SuccessRate*100), "green"))
	fmt.Printf("Total Duration: %s\n", stats.TotalDuration)
	fmt.Printf("Finished at: %s\n", time.Now().Format(time.RFC3339))
}

// GenerateReport 生成报告
func (cr *ConsoleReporter) GenerateReport() ([]byte, error) {
	return []byte("Console report completed"), nil
}

// colorize 着色文本
func (cr *ConsoleReporter) colorize(text, color string) string {
	if !cr.colors {
		return text
	}

	colors := map[string]string{
		"red":    "\033[31m",
		"green":  "\033[32m",
		"yellow": "\033[33m",
		"blue":   "\033[34m",
		"cyan":   "\033[36m",
		"reset":  "\033[0m",
	}

	if colorCode, exists := colors[color]; exists {
		return colorCode + text + colors["reset"]
	}
	return text
}

// XMLReporter 实现

// ReportStart 开始报告
func (xr *XMLReporter) ReportStart(stats *TestStats) {
	xr.report.Name = "NetCore-Go Test Report"
	xr.report.StartTime = time.Now()
	xr.report.Stats = stats
}

// ReportSuite 报告套件
func (xr *XMLReporter) ReportSuite(suite *TestSuite, stats *TestStats) {
	suiteReport := &SuiteReport{
		Name:        suite.Name,
		Description: suite.Description,
		Type:        suite.Type,
		Tags:        suite.Tags,
		StartTime:   time.Now(),
		Tests:       suite.Tests,
		Stats:       calculateSuiteStats(suite.Tests),
	}
	xr.report.Suites = append(xr.report.Suites, suiteReport)
}

// ReportTest 报告测试
func (xr *XMLReporter) ReportTest(test *TestCase) {
	// 测试结果已经在TestCase中
}

// ReportEnd 结束报告
func (xr *XMLReporter) ReportEnd(stats *TestStats) {
	xr.report.EndTime = time.Now()
	xr.report.Duration = xr.report.EndTime.Sub(xr.report.StartTime)
	xr.report.Stats = stats
}

// GenerateReport 生成XML报告
func (xr *XMLReporter) GenerateReport() ([]byte, error) {
	// 简化的XML生成
	xml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="%s" tests="%d" failures="%d" time="%.3f">
`,
		xr.report.Name, xr.report.Stats.TotalTests, xr.report.Stats.FailedTests, xr.report.Duration.Seconds())

	for _, suite := range xr.report.Suites {
		xml += fmt.Sprintf(`  <testsuite name="%s" tests="%d" failures="%d" time="%.3f">
`,
			suite.Name, suite.Stats.TotalTests, suite.Stats.FailedTests, suite.Duration.Seconds())

		for _, test := range suite.Tests {
			xml += fmt.Sprintf(`    <testcase name="%s" time="%.3f"`,
				test.Name, test.Duration.Seconds())

			if test.Status == TestStatusFailed {
				xml += fmt.Sprintf(`>
      <failure message="%s">%s</failure>
    </testcase>
`,
					test.Error, test.Error)
			} else if test.Status == TestStatusSkipped {
				xml += fmt.Sprintf(`>
      <skipped message="%s"/>
    </testcase>
`,
					test.SkipReason)
			} else {
				xml += "/>
"
			}
		}

		xml += "  </testsuite>\n"
	}

	xml += "</testsuites>\n"

	// 写入文件
	if xr.outputPath != "" {
		dir := filepath.Dir(xr.outputPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}

		if err := os.WriteFile(xr.outputPath, []byte(xml), 0644); err != nil {
			return nil, fmt.Errorf("failed to write XML report: %w", err)
		}
	}

	return []byte(xml), nil
}

// HTMLReporter 实现

// ReportStart 开始报告
func (hr *HTMLReporter) ReportStart(stats *TestStats) {
	hr.report.Name = "NetCore-Go Test Report"
	hr.report.StartTime = time.Now()
	hr.report.Stats = stats
}

// ReportSuite 报告套件
func (hr *HTMLReporter) ReportSuite(suite *TestSuite, stats *TestStats) {
	suiteReport := &SuiteReport{
		Name:        suite.Name,
		Description: suite.Description,
		Type:        suite.Type,
		Tags:        suite.Tags,
		StartTime:   time.Now(),
		Tests:       suite.Tests,
		Stats:       calculateSuiteStats(suite.Tests),
	}
	hr.report.Suites = append(hr.report.Suites, suiteReport)
}

// ReportTest 报告测试
func (hr *HTMLReporter) ReportTest(test *TestCase) {
	// 测试结果已经在TestCase中
}

// ReportEnd 结束报告
func (hr *HTMLReporter) ReportEnd(stats *TestStats) {
	hr.report.EndTime = time.Now()
	hr.report.Duration = hr.report.EndTime.Sub(hr.report.StartTime)
	hr.report.Stats = stats
}

// GenerateReport 生成HTML报告
func (hr *HTMLReporter) GenerateReport() ([]byte, error) {
	// 使用模板生成HTML报告
	reportJSON, err := json.Marshal(hr.report)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal report data: %w", err)
	}

	html := strings.ReplaceAll(hr.template, "{{REPORT_DATA}}", string(reportJSON))
	html = strings.ReplaceAll(html, "{{REPORT_TITLE}}", hr.report.Name)
	html = strings.ReplaceAll(html, "{{GENERATION_TIME}}", time.Now().Format(time.RFC3339))

	// 写入文件
	if hr.outputPath != "" {
		dir := filepath.Dir(hr.outputPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}

		if err := os.WriteFile(hr.outputPath, []byte(html), 0644); err != nil {
			return nil, fmt.Errorf("failed to write HTML report: %w", err)
		}
	}

	return []byte(html), nil
}

// 辅助函数

// calculateSuiteStats 计算套件统计
func calculateSuiteStats(tests []*TestCase) *SuiteStats {
	stats := &SuiteStats{}
	for _, test := range tests {
		stats.TotalTests++
		switch test.Status {
		case TestStatusPassed:
			stats.PassedTests++
		case TestStatusFailed:
			stats.FailedTests++
		case TestStatusSkipped:
			stats.SkippedTests++
		case TestStatusTimeout:
			stats.TimeoutTests++
		}
	}
	return stats
}

// collectEnvironmentInfo 收集环境信息
func collectEnvironmentInfo() *Environment {
	env := &Environment{
		Environment: make(map[string]string),
	}

	// 收集基本环境信息
	if hostname, err := os.Hostname(); err == nil {
		env.Hostname = hostname
	}

	if wd, err := os.Getwd(); err == nil {
		env.WorkingDir = wd
	}

	// 收集环境变量
	for _, envVar := range os.Environ() {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) == 2 {
			env.Environment[parts[0]] = parts[1]
		}
	}

	if tz, err := time.Now().Zone(); err == nil {
		env.Timezone = tz
	}

	return env
}

// defaultHTMLTemplate 默认HTML模板
const defaultHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{REPORT_TITLE}}</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .header { background: #f5f5f5; padding: 20px; border-radius: 5px; }
        .stats { display: flex; gap: 20px; margin: 20px 0; }
        .stat { background: #fff; padding: 15px; border-radius: 5px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .suite { margin: 20px 0; border: 1px solid #ddd; border-radius: 5px; }
        .suite-header { background: #f9f9f9; padding: 15px; border-bottom: 1px solid #ddd; }
        .test { padding: 10px 15px; border-bottom: 1px solid #eee; }
        .test:last-child { border-bottom: none; }
        .passed { color: #28a745; }
        .failed { color: #dc3545; }
        .skipped { color: #ffc107; }
        .timeout { color: #dc3545; }
    </style>
</head>
<body>
    <div class="header">
        <h1>{{REPORT_TITLE}}</h1>
        <p>Generated at: {{GENERATION_TIME}}</p>
    </div>
    
    <div id="report-content">
        <!-- Report content will be generated by JavaScript -->
    </div>
    
    <script>
        const reportData = {{REPORT_DATA}};
        
        function renderReport(data) {
            const content = document.getElementById('report-content');
            
            // Render stats
            const statsHtml = \`
                <div class="stats">
                    <div class="stat">
                        <h3>Total Tests</h3>
                        <p>\${data.stats.total_tests}</p>
                    </div>
                    <div class="stat">
                        <h3>Passed</h3>
                        <p class="passed">\${data.stats.passed_tests}</p>
                    </div>
                    <div class="stat">
                        <h3>Failed</h3>
                        <p class="failed">\${data.stats.failed_tests}</p>
                    </div>
                    <div class="stat">
                        <h3>Skipped</h3>
                        <p class="skipped">\${data.stats.skipped_tests}</p>
                    </div>
                    <div class="stat">
                        <h3>Success Rate</h3>
                        <p>\${(data.stats.success_rate * 100).toFixed(2)}%</p>
                    </div>
                </div>
            \`;
            
            // Render suites
            const suitesHtml = data.suites.map(suite => \`
                <div class="suite">
                    <div class="suite-header">
                        <h2>\${suite.name}</h2>
                        <p>\${suite.description}</p>
                        <p>Type: \${suite.type} | Tests: \${suite.stats.total_tests}</p>
                    </div>
                    \${suite.tests.map(test => \`
                        <div class="test">
                            <span class="\${test.status}">\${test.status.toUpperCase()}</span>
                            <strong>\${test.name}</strong>
                            <span>(\${test.duration})</span>
                            \${test.error ? \`<br><small class="failed">\${test.error}</small>\` : ''}
                        </div>
                    \`).join('')}
                </div>
            \`).join('');
            
            content.innerHTML = statsHtml + suitesHtml;
        }
        
        renderReport(reportData);
    </script>
</body>
</html>`