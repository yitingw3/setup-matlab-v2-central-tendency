package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gonum.org/v1/gonum/stat"
)

type GithubActionRunsResponse struct {
	TotalCount   int `json:"total_count"`
	WorkflowRuns []struct {
		JobsUrl string `json:"jobs_url"`
		// You can add more fields here if needed
	} `json:"workflow_runs"`
}

type GithubActionJobsResponse struct {
	TotalCount int   `json:"total_count"`
	Jobs       []Job `json:"jobs"`
}

type Job struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`
	Labels        []string `json:"labels"`
	Status        string   `json:"status"`
	Conclusion    string   `json:"conclusion"`
	StartTime     string   `json:"started_at"`
	CompletedTime string   `json:"completed_at"`
	RunTime       float64  ``
	Steps         []Step   `json:"steps"`
}

type Step struct {
	Name          string  `json:"name"`
	Status        string  `json:"status"`
	Conclusion    string  `json:"conclusion"`
	StartTime     string  `json:"started_at"`
	CompletedTime string  `json:"completed_at"`
	RunTime       float64 ``
}

var (
	runsRes        GithubActionRunsResponse
	jobsRes        GithubActionJobsResponse
	runtimeMacos   []float64
	runtimeUbuntu  []float64
	runtimeWindows []float64
)

var gitToken = getToken("GIT_TOKEN")

func getWorkflowRunsData(per_page int, page int) {
	// Filtered workflows should meet:
	// 1. running on hourly branch
	// 2. status is success
	// 3. created after 2023-04-12
	url := "https://api.github.com/repos/mathworks/ci-configuration-examples/actions/runs?created:%3E2023-04-12&branch=hourly&status=success" +
		"&per_page" + strconv.Itoa(per_page) +
		"&page=" + strconv.Itoa(page)
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Add("Authorization", gitToken)
	res, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	// Error out if not a successful respond
	if res.StatusCode != 200 {
		fmt.Println("Received non-200 status code from wrokflow api:", res.StatusCode)
	}
	defer res.Body.Close()

	err = json.NewDecoder(res.Body).Decode(&runsRes)
	if err != nil {
		panic(err)
	}
}

func getToken(tokenName string) string {
	// Load token from .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	gitToken := os.Getenv(tokenName)
	return gitToken
}

func getJobsData(jobsUrl string) {

	client := &http.Client{}
	req, err := http.NewRequest("GET", jobsUrl, nil)
	req.Header.Add("Authorization", gitToken)
	if err != nil {
		panic(err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")

	res, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	// Error out if not a successful respond
	if res.StatusCode != 200 {
		fmt.Println("Received non-200 status code from job data api:", res.StatusCode)
	}
	defer res.Body.Close()

	// Decode the JSON response
	err = json.NewDecoder(res.Body).Decode(&jobsRes)
	if err != nil {
		panic(err)
	}
	//fmt.Println(jobsRes.TotalCount)

	for i := range jobsRes.Jobs {
		job := &jobsRes.Jobs[i]
		// Filter jobs using setup-matlab-v2-beta
		if strings.Contains(job.Name, "build-v1") {
			continue
		}
		//fmt.Println("=================================")
		//fmt.Println("Job Name: ", job.Name)
		// Get info of setup-matlab step
		step := &job.Steps[2]
		//fmt.Println("=============================")
		//fmt.Println("  Step Name: ", step.Name)
		//fmt.Println("  Step Status: ", step.Status)
		runtime := getRuntime(step.StartTime, step.CompletedTime)
		step.RunTime = runtime
		//fmt.Println("  Step Runtime: ", step.RunTime)

		switch {
		case strings.Contains(job.Name, "macos-12"):
			runtimeMacos = append(runtimeMacos, step.RunTime)
		case strings.Contains(job.Name, "windows-2022"):
			runtimeWindows = append(runtimeWindows, step.RunTime)
		case strings.Contains(job.Name, "ubuntu-22.04"):
			runtimeUbuntu = append(runtimeUbuntu, step.RunTime)
		default:
			fmt.Println("Os should be one of {macos, windows-2022, ubuntu-22.04}")
		}

	}

}

func getRuntime(startedAt string, completedAt string) float64 {
	//Caulculate run duration time
	startedAtTime, err := time.Parse(time.RFC3339, startedAt)
	if err != nil {
		log.Fatal(err)
	}
	completedAtTime, err := time.Parse(time.RFC3339, completedAt)
	if err != nil {
		log.Fatal(err)
	}
	runtime := float64(completedAtTime.Sub(startedAtTime) / time.Second)
	return runtime
}

func median(data []float64) (float64, float64, float64) {
	dataCopy := make([]float64, len(data))
	copy(dataCopy, data)

	sort.Float64s(dataCopy)

	var median float64
	l := len(dataCopy)
	if l == 0 {
		fmt.Println("dataset is empty")
	} else if l%2 == 0 {
		median = (dataCopy[l/2-1] + dataCopy[l/2]) / 2
	} else {
		median = dataCopy[l/2]
	}

	min := dataCopy[0]
	max := dataCopy[len(data)-1]

	return min, max, median
}

func calculateStat(data []float64) {
	// Calcaulate the mean, median, min, max, and standard deviation

	// Calculate the mean
	mean := stat.Mean(data, nil)

	// Calculate the min, max and median
	min, max, median := median(data)

	// Calculate the standard deviation
	stdDev := stat.StdDev(data, nil)

	fmt.Printf("Mean: %.2f\n", mean)
	fmt.Printf("Median: %.2f\n", median)
	fmt.Printf("Min: %.2f\n", min)
	fmt.Printf("Max: %.2f\n", max)
	fmt.Printf("Standard Deviation: %.2f\n", stdDev)
}

func main() {
	// Set the number of results per_page(max 100)
	// Calculate the page number we can fetch
	per_page := 100
	pageMax := int(math.Ceil(float64(1000 / per_page)))

	for page := 1; page <= pageMax; page++ {
		getWorkflowRunsData(per_page, page)
		// If the number of result return is 0, break the loop
		if runsRes.TotalCount == 0 {
			break
		}
		for j := range runsRes.WorkflowRuns {
			jobsUrl := runsRes.WorkflowRuns[j].JobsUrl
			//fmt.Println(jobsUrl)
			//get each job data and unmarshall
			getJobsData(jobsUrl)
		}

	}

	fmt.Println("==============macos===============")
	calculateStat(runtimeMacos)
	fmt.Println("=============windows==============")
	calculateStat(runtimeWindows)
	fmt.Println("==============Ubuntu===============")
	calculateStat(runtimeUbuntu)

}
