package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"

	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
)

const defaultConnectionString = ""

func main() {
	// check number of args
	if len(os.Args) < 3 {
		fmt.Println("Usage: gh <TICKER> <#ofTablePartitionKeys>")
		os.Exit(1)
	}

	// parse
	ticker := os.Args[1]
	numRecordsStr := os.Args[2]
	numRecords, err := strconv.Atoi(numRecordsStr)
	if err != nil {
		log.Fatalf("Invalid number of records: %v", err)
	}

	// use env variable
	connStr := os.Getenv("AZURE_STORAGE_CONNECTION_STRING")
	if connStr == "" {
		log.Println("Environment variable AZURE_STORAGE_CONNECTION_STRING not set, using default connection string.")
		connStr = defaultConnectionString
	}

	// connct acure tables
	serviceClient, err := aztables.NewServiceClientFromConnectionString(connStr, nil)
	if err != nil {
		log.Fatalf("Failed to create Table service client: %v", err)
	}

	client := serviceClient.NewClient("FundingRates")

	fundingRates, timestamps, err := queryFundingRates(client, ticker, numRecords)
	if err != nil {
		log.Fatalf("Error querying table: %v", err)
	}

	outputFile := "funding_graph.html"
	if err := generateGraph(ticker, fundingRates, timestamps, outputFile); err != nil {
		log.Fatalf("Error generating graph: %v", err)
	}

	fmt.Println("Graph successfully created:", outputFile)

	// live server
	startServer(outputFile)
	fmt.Println("Application finished.")
}

func queryFundingRates(client *aztables.Client, ticker string, limit int) ([]float64, []string, error) {
	filter := fmt.Sprintf("PartitionKey eq '%s'", ticker)

	// query entities
	pager := client.NewListEntitiesPager(&aztables.ListEntitiesOptions{
		Filter: &filter,
	})

	var entities []map[string]interface{}

	fmt.Printf("Querying Azure Table Storage for ticker: %s\n", ticker)
	for pager.More() {
		resp, err := pager.NextPage(context.Background())
		if err != nil {
			return nil, nil, fmt.Errorf("failed to fetch page: %w", err)
		}

		for _, rawEntity := range resp.Entities {
			// unmarshal the entity into a map
			entity := make(map[string]interface{})
			if err := json.Unmarshal(rawEntity, &entity); err != nil {
				return nil, nil, fmt.Errorf("failed to unmarshal entity: %w", err)
			}
			entities = append(entities, entity)
		}
	}

	sort.Slice(entities, func(i, j int) bool {
		return entities[i]["RowKey"].(string) < entities[j]["RowKey"].(string)
	})

	if len(entities) > limit {
		entities = entities[len(entities)-limit:]
	}

	var fundingRates []float64
	var timestamps []string
	for _, entity := range entities {
		if funding, ok := entity["funding_rate"].(string); ok {
			fundingValue, err := strconv.ParseFloat(funding, 64)
			if err == nil {
				fundingRates = append(fundingRates, fundingValue)
			}
		}
		if rowKey, ok := entity["RowKey"].(string); ok {
			timestamps = append(timestamps, rowKey)
		}
	}

	return fundingRates, timestamps, nil
}

func generateGraph(ticker string, fundingRates []float64, timestamps []string, outputFile string) error {
	// line chart
	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{Title: fmt.Sprintf("Funding Rate Over Time for %s", ticker)}),
		charts.WithXAxisOpts(opts.XAxis{Name: "Timestamp"}),
		charts.WithYAxisOpts(opts.YAxis{Name: "Funding Rate"}),
	)

	line.SetXAxis(timestamps).AddSeries("Funding Rates", generateLineItems(fundingRates))

	// html file save
	f, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer f.Close()
	return line.Render(f)
}

func generateLineItems(data []float64) []opts.LineData {
	items := make([]opts.LineData, len(data))
	for i, d := range data {
		items[i] = opts.LineData{Value: d}
	}
	return items
}

// start html servew
func startServer(file string) {
	server := &http.Server{Addr: ":8080"}

	// stop server
	done := make(chan bool)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Request received for:", r.URL.Path)
		http.ServeFile(w, r, file)

		go func() {
			fmt.Println("Shutting down the server...")
			done <- true
		}()
	})

	go func() {
		fmt.Println("Serving", file, "at http://localhost:8080")
		openBrowser("http://localhost:8080") // Open the browser
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// wait for stop[]
	<-done

	if err := server.Shutdown(context.Background()); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}
}

func openBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	if err != nil {
		log.Printf("Failed to open browser: %v", err)
	}
}
