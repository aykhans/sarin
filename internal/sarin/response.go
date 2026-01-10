package sarin

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"go.yaml.in/yaml/v4"
)

const DefaultResponseDurationAccuracy uint32 = 1
const DefaultResponseColumnMaxWidth = 50

// Duration wraps time.Duration to provide consistent JSON/YAML marshaling as human-readable strings.
type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	//nolint:wrapcheck
	return json.Marshal(time.Duration(d).String())
}

func (d Duration) MarshalYAML() (any, error) {
	return time.Duration(d).String(), nil
}

func (d Duration) String() string {
	dur := time.Duration(d)
	switch {
	case dur >= time.Second:
		return dur.Round(time.Millisecond).String()
	case dur >= time.Millisecond:
		return dur.Round(time.Microsecond).String()
	default:
		return dur.String()
	}
}

// BigInt wraps big.Int to provide consistent JSON/YAML marshaling as numbers.
type BigInt struct {
	*big.Int
}

func (b BigInt) MarshalJSON() ([]byte, error) {
	return []byte(b.Int.String()), nil
}

func (b BigInt) MarshalYAML() (any, error) {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!int",
		Value: b.Int.String(),
	}, nil
}

func (b BigInt) String() string {
	return b.Int.String()
}

type Response struct {
	durations map[time.Duration]uint64
}

type SarinResponseData struct {
	sync.Mutex

	Responses map[string]*Response

	// accuracy is the time bucket size in nanoseconds for storing response durations.
	// Larger values (e.g., 1000) save memory but reduce accuracy by grouping more durations together.
	// Smaller values (e.g., 10) improve accuracy but increase memory usage.
	// Minimum value is 1 (most accurate, highest memory usage).
	// Default value is 1.
	accuracy time.Duration
}

func NewSarinResponseData(accuracy uint32) *SarinResponseData {
	if accuracy == 0 {
		accuracy = DefaultResponseDurationAccuracy
	}

	return &SarinResponseData{
		Responses: make(map[string]*Response),
		accuracy:  time.Duration(accuracy),
	}
}

func (data *SarinResponseData) Add(responseKey string, responseTime time.Duration) {
	data.Lock()
	defer data.Unlock()

	response, ok := data.Responses[responseKey]
	if !ok {
		data.Responses[responseKey] = &Response{
			durations: map[time.Duration]uint64{
				responseTime / data.accuracy: 1,
			},
		}
	} else {
		response.durations[responseTime/data.accuracy]++
	}
}

func (data *SarinResponseData) PrintTable() {
	data.Lock()
	defer data.Unlock()

	output := data.prepareOutputData()

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("246")).
		Padding(0, 1)

	cellStyle := lipgloss.NewStyle().
		Padding(0, 1)

	rows := make([][]string, 0, len(output.Responses)+1)
	for key, stats := range output.Responses {
		rows = append(rows, []string{
			wrapText(key, DefaultResponseColumnMaxWidth),
			stats.Count.String(),
			stats.Min.String(),
			stats.Max.String(),
			stats.Average.String(),
			stats.P90.String(),
			stats.P95.String(),
			stats.P99.String(),
		})
	}

	rows = append(rows, []string{
		"Total",
		output.Total.Count.String(),
		output.Total.Min.String(),
		output.Total.Max.String(),
		output.Total.Average.String(),
		output.Total.P90.String(),
		output.Total.P95.String(),
		output.Total.P99.String(),
	})

	tbl := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("240"))).
		BorderRow(true).
		Headers("Response", "Count", "Min", "Max", "Average", "P90", "P95", "P99").
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		})

	fmt.Println(tbl)
}

func (data *SarinResponseData) PrintJSON() {
	data.Lock()
	defer data.Unlock()

	output := data.prepareOutputData()
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		panic(err)
	}
}

func (data *SarinResponseData) PrintYAML() {
	data.Lock()
	defer data.Unlock()

	output := data.prepareOutputData()
	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	if err := encoder.Encode(output); err != nil {
		panic(err)
	}
}

type responseStat struct {
	Count   BigInt   `json:"count"   yaml:"count"`
	Min     Duration `json:"min"     yaml:"min"`
	Max     Duration `json:"max"     yaml:"max"`
	Average Duration `json:"average" yaml:"average"`
	P90     Duration `json:"p90"     yaml:"p90"`
	P95     Duration `json:"p95"     yaml:"p95"`
	P99     Duration `json:"p99"     yaml:"p99"`
}

type responseStats map[string]responseStat

type outputData struct {
	Responses map[string]responseStat `json:"responses" yaml:"responses"`
	Total     responseStat            `json:"total"     yaml:"total"`
}

func (data *SarinResponseData) prepareOutputData() outputData {
	switch len(data.Responses) {
	case 0:
		return outputData{
			Responses: make(map[string]responseStat),
			Total:     responseStat{},
		}
	case 1:
		var (
			responseKey string
			stats       responseStat
		)
		for key, response := range data.Responses {
			stats = calculateStats(response.durations, data.accuracy)
			responseKey = key
		}
		return outputData{
			Responses: responseStats{
				responseKey: stats,
			},
			Total: stats,
		}
	default:
		// Calculate stats for each response
		allStats := make(responseStats)
		var totalDurations = make(map[time.Duration]uint64)

		for key, response := range data.Responses {
			stats := calculateStats(response.durations, data.accuracy)
			allStats[key] = stats

			// Aggregate for total row
			for duration, count := range response.durations {
				totalDurations[duration] += count
			}
		}

		return outputData{
			Responses: allStats,
			Total:     calculateStats(totalDurations, data.accuracy),
		}
	}
}

func calculateStats(durations map[time.Duration]uint64, accuracy time.Duration) responseStat {
	if len(durations) == 0 {
		return responseStat{}
	}

	// Extract and sort unique durations
	sortedDurations := make([]time.Duration, 0, len(durations))
	for duration := range durations {
		sortedDurations = append(sortedDurations, duration)
	}
	slices.Sort(sortedDurations)

	sum := new(big.Int)
	totalCount := new(big.Int)
	minDuration := sortedDurations[0] * accuracy
	maxDuration := sortedDurations[len(sortedDurations)-1] * accuracy

	for _, duration := range sortedDurations {
		actualDuration := duration * accuracy
		count := durations[duration]

		totalCount.Add(
			totalCount,
			new(big.Int).SetUint64(count),
		)

		sum.Add(
			sum,
			new(big.Int).Mul(
				new(big.Int).SetInt64(int64(actualDuration)),
				new(big.Int).SetUint64(count),
			),
		)
	}

	// Calculate percentiles
	p90 := calculatePercentile(sortedDurations, durations, totalCount, 90, accuracy)
	p95 := calculatePercentile(sortedDurations, durations, totalCount, 95, accuracy)
	p99 := calculatePercentile(sortedDurations, durations, totalCount, 99, accuracy)

	return responseStat{
		Count:   BigInt{totalCount},
		Min:     Duration(minDuration),
		Max:     Duration(maxDuration),
		Average: Duration(div(sum, totalCount).Int64()),
		P90:     p90,
		P95:     p95,
		P99:     p99,
	}
}

func calculatePercentile(sortedDurations []time.Duration, durations map[time.Duration]uint64, totalCount *big.Int, percentile int, accuracy time.Duration) Duration {
	// Calculate the target position for the percentile
	// Using ceiling method: position = ceil(totalCount * percentile / 100)
	target := new(big.Int).Mul(totalCount, big.NewInt(int64(percentile)))
	target.Add(target, big.NewInt(99)) // Add 99 to achieve ceiling division by 100
	target.Div(target, big.NewInt(100))

	// Accumulate counts until we reach the target position
	cumulative := new(big.Int)
	for _, duration := range sortedDurations {
		count := durations[duration]
		cumulative.Add(cumulative, new(big.Int).SetUint64(count))

		if cumulative.Cmp(target) >= 0 {
			return Duration(duration * accuracy)
		}
	}

	// Fallback to the last duration (shouldn't happen with valid data)
	return Duration(sortedDurations[len(sortedDurations)-1] * accuracy)
}

// div performs division with rounding to the nearest integer.
func div(x, y *big.Int) *big.Int {
	quotient, remainder := new(big.Int).DivMod(x, y, new(big.Int))
	if remainder.Mul(remainder, big.NewInt(2)).Cmp(y) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	return quotient
}

// wrapText wraps a string to multiple lines if it exceeds maxWidth.
func wrapText(s string, maxWidth int) string {
	if len(s) <= maxWidth {
		return s
	}

	var lines []string
	for len(s) > maxWidth {
		lines = append(lines, s[:maxWidth])
		s = s[maxWidth:]
	}
	if len(s) > 0 {
		lines = append(lines, s)
	}

	return strings.Join(lines, "\n")
}
