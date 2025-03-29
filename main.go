package main

import (
	"fmt"
	"math"
	"math/rand"
	"syscall/js"
)

// Model parameters
type SIRParams struct {
	R0                   float64 // Basic reproduction number
	InfectiousDuration   float64 // Duration of infectious period in days
	MortalityRate        float64 // Proportion of infected that die
	ExternalIntroduction float64 // Probability of external introduction
	BiosecurityLevel     float64 // Level of biosecurity (0-1)
	VaccinationRate      float64 // Proportion of birds vaccinated
	TotalPopulation      int     // Total bird population
	SimulationDays       int     // Number of days to simulate
	Budget               float64 // Available budget for biosecurity and vaccination
}

// SIR model compartments
type SIRState struct {
	Susceptible int
	Infectious  int
	Recovered   int
	Dead        int
	Day         int
}

// Default parameters
func DefaultParams() SIRParams {
	return SIRParams{
		R0:                   3.0,
		InfectiousDuration:   3.0,
		MortalityRate:        0.95,
		ExternalIntroduction: 0.20,
		BiosecurityLevel:     0.0,
		VaccinationRate:      0.0,
		TotalPopulation:      400000,
		SimulationDays:       100,
		Budget:               250000,
	}
}

// Calculate costs and validate budget
func validateBudget(params SIRParams) error {
	biosecurityCost := (params.BiosecurityLevel * 100000)                       // $10000 per 10%
	vaccinationCost := float64(params.TotalPopulation) * params.VaccinationRate // $1 per bird
	totalCost := biosecurityCost + vaccinationCost

	if totalCost > params.Budget {
		return fmt.Errorf("total cost ($%.2f) exceeds budget ($%.2f)", totalCost, params.Budget)
	}
	return nil
}

// Run a single stochastic SIR simulation
func runSimulation(params SIRParams) []SIRState {
	// Calculate effective parameters
	effectiveExternalIntro := params.ExternalIntroduction * (1.0 - params.BiosecurityLevel)

	// Initialize compartments
	vaccinatedCount := int(float64(params.TotalPopulation) * params.VaccinationRate)
	initialSusceptible := params.TotalPopulation - vaccinatedCount

	// Daily transmissibility based on R0 and infectious duration
	beta := params.R0 / params.InfectiousDuration

	// Initialize results slice with initial state
	results := []SIRState{
		{
			Susceptible: initialSusceptible,
			Infectious:  0,
			Recovered:   vaccinatedCount,
			Dead:        0,
			Day:         0,
		},
	}

	// Run simulation for specified number of days
	for day := 1; day <= params.SimulationDays; day++ {
		prevState := results[len(results)-1]

		// Check for external introduction if there are no infectious birds
		if prevState.Infectious == 0 {
			if rand.Float64() < effectiveExternalIntro {
				// Introduce one infectious bird
				prevState.Infectious = 1
				prevState.Susceptible--
			}
		}

		// Calculate new infections (stochastic)
		newInfections := 0
		if prevState.Infectious > 0 && prevState.Susceptible > 0 {
			lambda := beta * float64(prevState.Infectious) * float64(prevState.Susceptible) / float64(params.TotalPopulation)
			newInfections = poissonRandom(lambda)
			newInfections = min(newInfections, prevState.Susceptible)
		}

		// Calculate recoveries and deaths (stochastic)
		recoveryRate := 1.0 / params.InfectiousDuration
		expectedRecoveries := float64(prevState.Infectious) * recoveryRate
		totalExitingInfectious := poissonRandom(expectedRecoveries)
		totalExitingInfectious = min(totalExitingInfectious, prevState.Infectious)

		// Split exits between recoveries and deaths based on mortality rate
		newDeaths := binomialRandom(totalExitingInfectious, params.MortalityRate)
		newRecoveries := totalExitingInfectious - newDeaths

		// Update compartments
		newState := SIRState{
			Susceptible: prevState.Susceptible - newInfections,
			Infectious:  prevState.Infectious + newInfections - totalExitingInfectious,
			Recovered:   prevState.Recovered + newRecoveries,
			Dead:        prevState.Dead + newDeaths,
			Day:         day,
		}

		results = append(results, newState)
	}

	return results
}

// Run multiple simulations and average results
func runMultipleSimulations(params SIRParams, numSimulations int) ([]map[string]float64, error) {
	// Validate budget first
	if err := validateBudget(params); err != nil {
		return nil, err
	}

	// Initialize result storage
	aggregatedResults := make([]map[string]float64, params.SimulationDays+1)
	for i := 0; i <= params.SimulationDays; i++ {
		aggregatedResults[i] = map[string]float64{
			"day":         float64(i),
			"susceptible": 0,
			"infectious":  0,
			"recovered":   0,
			"dead":        0,
		}
	}

	// Run multiple simulations
	for i := 0; i < numSimulations; i++ {
		results := runSimulation(params)

		// Aggregate results
		for j, state := range results {
			aggregatedResults[j]["susceptible"] += float64(state.Susceptible) / float64(numSimulations)
			aggregatedResults[j]["infectious"] += float64(state.Infectious) / float64(numSimulations)
			aggregatedResults[j]["recovered"] += float64(state.Recovered) / float64(numSimulations)
			aggregatedResults[j]["dead"] += float64(state.Dead) / float64(numSimulations)
		}
	}

	// Calculate final profit
	averageFinalState := aggregatedResults[len(aggregatedResults)-1]
	totalLiveBirds := averageFinalState["susceptible"] + averageFinalState["recovered"]
	profit := totalLiveBirds * 3.0 // $3 per live bird

	// Add costs
	biosecurityCost := params.BiosecurityLevel * 100000
	vaccinationCost := float64(params.TotalPopulation) * params.VaccinationRate
	remainingBudget := params.Budget - biosecurityCost - vaccinationCost
	netProfit := profit - biosecurityCost - vaccinationCost + remainingBudget // Add remaining budget to net profit

	// Add financial results to final day
	aggregatedResults[len(aggregatedResults)-1]["profit"] = profit
	aggregatedResults[len(aggregatedResults)-1]["costs"] = biosecurityCost + vaccinationCost
	aggregatedResults[len(aggregatedResults)-1]["netProfit"] = netProfit

	return aggregatedResults, nil
}

// Helper function for Poisson random numbers
func poissonRandom(lambda float64) int {
	if lambda <= 0 {
		return 0
	}

	// For large lambda, use normal approximation
	if lambda > 100 {
		// Normal approximation with continuity correction
		return int(rand.NormFloat64()*math.Sqrt(lambda) + lambda + 0.5)
	}

	// For small lambda, use Knuth's algorithm
	L := math.Exp(-lambda)
	k := 0
	p := 1.0

	for p > L {
		k++
		p *= rand.Float64()
	}

	return k - 1
}

// Helper function for binomial random numbers
func binomialRandom(n int, p float64) int {
	if n <= 0 || p <= 0 {
		return 0
	}
	if p >= 1 {
		return n
	}

	count := 0
	for i := 0; i < n; i++ {
		if rand.Float64() < p {
			count++
		}
	}

	return count
}

// Helper function for min of two ints
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Convert simulation results to JS array
func resultsToJSValue(results []map[string]float64) js.Value {
	jsArray := js.Global().Get("Array").New(len(results))

	for i, day := range results {
		dayObj := js.Global().Get("Object").New()
		for key, value := range day {
			dayObj.Set(key, value)
		}
		jsArray.SetIndex(i, dayObj)
	}

	return jsArray
}

// JS wrapper function to run simulations
func runSimulationsJS() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		// Parse parameters from JavaScript
		jsParams := args[0]
		numSimulations := 10
		if len(args) > 1 {
			numSimulations = args[1].Int()
		}

		params := DefaultParams()

		// Update parameters if provided in JS object
		if !jsParams.IsUndefined() && !jsParams.IsNull() {
			if jsVal := jsParams.Get("r0"); !jsVal.IsUndefined() {
				params.R0 = jsVal.Float()
			}
			if jsVal := jsParams.Get("infectiousDuration"); !jsVal.IsUndefined() {
				params.InfectiousDuration = jsVal.Float()
			}
			if jsVal := jsParams.Get("mortalityRate"); !jsVal.IsUndefined() {
				params.MortalityRate = jsVal.Float()
			}
			if jsVal := jsParams.Get("externalIntroduction"); !jsVal.IsUndefined() {
				params.ExternalIntroduction = jsVal.Float()
			}
			if jsVal := jsParams.Get("biosecurityLevel"); !jsVal.IsUndefined() {
				params.BiosecurityLevel = jsVal.Float()
			}
			if jsVal := jsParams.Get("vaccinationRate"); !jsVal.IsUndefined() {
				params.VaccinationRate = jsVal.Float()
			}
			if jsVal := jsParams.Get("totalPopulation"); !jsVal.IsUndefined() {
				params.TotalPopulation = jsVal.Int()
			}
			if jsVal := jsParams.Get("simulationDays"); !jsVal.IsUndefined() {
				params.SimulationDays = jsVal.Int()
			}
		}

		// Run the simulations
		results, err := runMultipleSimulations(params, numSimulations)
		if err != nil {
			// Create error object for JavaScript
			errorObj := js.Global().Get("Object").New()
			errorObj.Set("error", err.Error())
			return errorObj
		}

		// Return results as JS array
		return resultsToJSValue(results)
	})
}

// Get default parameters as JS object
func getDefaultParamsJS() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		params := DefaultParams()

		jsObj := js.Global().Get("Object").New()
		jsObj.Set("r0", params.R0)
		jsObj.Set("infectiousDuration", params.InfectiousDuration)
		jsObj.Set("mortalityRate", params.MortalityRate)
		jsObj.Set("externalIntroduction", params.ExternalIntroduction)
		jsObj.Set("biosecurityLevel", params.BiosecurityLevel)
		jsObj.Set("vaccinationRate", params.VaccinationRate)
		jsObj.Set("totalPopulation", params.TotalPopulation)
		jsObj.Set("simulationDays", params.SimulationDays)

		return jsObj
	})
}

func main() {
	fmt.Println("SIR Model WebAssembly module initialized")

	// Initialize global random number generator

	// Register functions
	js.Global().Set("runSimulations", runSimulationsJS())
	js.Global().Set("getDefaultParams", getDefaultParamsJS())

	// Keep the Go program running
	<-make(chan bool)
}
