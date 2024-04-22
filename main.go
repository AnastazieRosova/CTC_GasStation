// Gas station simulation

//1. Cars arrive at the gas station and wait in the queue for the free station
//2. Total number of cars and their arrival time is configurable
//3. There are 4 types of stations: gas, diesel, LPG, electric
//4. Count of stations and their serve time is configurable as interval (e.g. 2-5s) and can be different for each type
//5. Each station can serve only one car at a time, serving time is chosen randomly from station's interval
//6. After the car is served, it goes to the cash register.
//7. Count of cash registers and their handle time is configurable
//8. After the car is handled (random time from register handle time range) by cast register, it leaves the station.
//9. Program collects statistics about the time spent in the queue, time spent at the station and time spent at the cash
//10. register for every car
//11. Program prints the aggregate statistics at the end of the simulation

package main

import (
	"fmt"
	"math"
	"math/rand"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

var time_mode = time.Millisecond

var config = map[string]*struct {
	num            int
	serve_time_min time.Duration
	serve_time_max time.Duration
}{
	"gas":      {num: 2, serve_time_min: 2 * time_mode, serve_time_max: 5 * time_mode},
	"diesel":   {num: 2, serve_time_min: 3 * time_mode, serve_time_max: 6 * time_mode},
	"lpg":      {num: 1, serve_time_min: 4 * time_mode, serve_time_max: 7 * time_mode},
	"electric": {num: 1, serve_time_min: 5 * time_mode, serve_time_max: 10 * time_mode},
	"register": {num: 2, serve_time_min: 1 * time_mode, serve_time_max: 3 * time_mode},
	"car":      {num: 1500, serve_time_min: 1 * time_mode, serve_time_max: 2 * time_mode},
}

type Fuel struct {
	name string
}

type Station struct {
	fuel           Fuel
	id             string
	serve_time_min time.Duration
	serve_time_max time.Duration
	numVehicle     int
}

type Register struct {
	handle_time_min time.Duration
	handle_time_max time.Duration
}

type Vehicle struct {
	id                           int
	fuel                         Fuel
	time_start_in_register_queue time.Time
	time_start_in_station_queue  time.Time
}

func createVehicle(id int) Vehicle {
	fuels := getSupportedFuels()
	fuel := Fuel{name: fuels[rand.Intn(len(fuels))]}
	vehicle := Vehicle{fuel: fuel, id: id}
	return vehicle
}

func getSupportedFuels() []string {
	return []string{"gas", "diesel", "lpg", "electric"}
}

func initGasStation() ([]Station, []Register) {

	var stations []Station
	var station Station
	fuels := getSupportedFuels()
	for obj_type, values := range config {
		if slices.Contains(fuels, obj_type) {
			type_of_fuel := obj_type
			for i := 0; i < values.num; i++ {
				id := type_of_fuel + "_" + strconv.Itoa(i+1)
				station = Station{
					fuel:           Fuel{name: type_of_fuel},
					id:             id,
					serve_time_min: values.serve_time_min,
					serve_time_max: values.serve_time_max,
				}
				stations = append(stations, station)
			}
		}
	}

	var registers []Register
	for i := 0; i < config["register"].num; i++ {
		register := Register{
			handle_time_min: config["register"].serve_time_min,
			handle_time_max: config["register"].serve_time_max,
		}
		registers = append(registers, register)
	}

	return stations, registers
}

func getShortestQueueStation(station_queues map[Station]chan Vehicle, car_fuel Fuel) Station {
	shortest_queue_len := math.MaxInt64
	var shortest_station Station
	for station, queue := range station_queues {
		if station.fuel.name == car_fuel.name {
			if len(queue) < shortest_queue_len {
				shortest_queue_len = len(queue)
				shortest_station = station
			}
		}

	}
	return shortest_station
}
func getShortestQueueRegister(queues map[int]chan Vehicle) int {
	shortest := math.MaxInt64
	shortestIndex := -1
	for i, q := range queues {
		length := len(q)
		if length < shortest {
			shortest = length
			shortestIndex = i
		}
	}
	return shortestIndex
}

func simulateGasStation() {
	var stats = map[string]*struct {
		total_car        int
		total_time       time.Duration
		total_queue_time time.Duration
		max_queue_time   time.Duration
	}{
		"gas":      {total_car: 0, total_time: 0, total_queue_time: 0, max_queue_time: -1},
		"diesel":   {total_car: 0, total_time: 0, total_queue_time: 0, max_queue_time: -1},
		"lpg":      {total_car: 0, total_time: 0, total_queue_time: 0, max_queue_time: -1},
		"electric": {total_car: 0, total_time: 0, total_queue_time: 0, max_queue_time: -1},
		"register": {total_car: 0, total_time: 0, total_queue_time: 0, max_queue_time: -1},
	}

	wg := sync.WaitGroup{}
	num_cars := config["car"].num
	wg.Add(num_cars)
	var mu sync.Mutex

	stations, registers := initGasStation()
	printGasstationInitInfo(stations, registers)

	var station_queues = map[Station]chan Vehicle{}
	for _, station := range stations {
		station_queues[station] = make(chan Vehicle, num_cars)
	}
	register_queues := map[int]chan Vehicle{}
	for i, _ := range registers {
		register_queues[i] = make(chan Vehicle, num_cars)
	}

	// vytvareni aut a zarazni do fronty k prislusne stanici
	go func() {
		for i := 0; i < num_cars; i++ {
			vehicle := createVehicle(i + 1)
			vehicle.time_start_in_station_queue = time.Now()
			shortest_station := getShortestQueueStation(station_queues, vehicle.fuel)
			station_queues[shortest_station] <- vehicle
			printVehicleArrival(vehicle, shortest_station.id)
			time.Sleep(time.Duration(rand.Intn(4)) * time_mode)
		}
	}()

	// tankovani a zarazení k platbe
	for _, station := range stations {
		go func(station Station) {
			for {
				vehicle := <-station_queues[station]
				printVehicleStartRefuels(vehicle, station.id)
				mu.Lock()
				rnd_serve_time := time.Duration(float32(station.serve_time_min/time_mode) + rand.Float32()*float32(station.serve_time_max/time_mode-station.serve_time_min/time_mode))

				stats[vehicle.fuel.name].total_car += 1
				stats[vehicle.fuel.name].total_time += rnd_serve_time * time_mode
				time_in_que_station := time.Since(vehicle.time_start_in_station_queue)
				if time_in_que_station > stats[vehicle.fuel.name].max_queue_time {
					stats[vehicle.fuel.name].max_queue_time = time_in_que_station
				}
				stats[vehicle.fuel.name].total_queue_time += time_in_que_station

				time.Sleep(rnd_serve_time * time_mode)
				idx := getShortestQueueRegister(register_queues)
				vehicle.time_start_in_register_queue = time.Now()
				printVehicleEndRefuels(vehicle, station.id, idx)
				register_queues[idx] <- vehicle
				mu.Unlock()
			}
		}(station)
	}

	// platba
	for idx, register := range registers {
		go func(register Register, idx int) {
			for {
				vehicle := <-register_queues[idx]
				mu.Lock()
				rnd_handle_time := time.Duration(float32(register.handle_time_min/time_mode) + rand.Float32()*(float32(register.handle_time_max/time_mode)-float32(register.handle_time_min/time_mode)))

				stats["register"].total_car += 1
				stats["register"].total_time += rnd_handle_time * time_mode
				time_in_que_register := time.Since(vehicle.time_start_in_register_queue)
				if time_in_que_register > stats["register"].max_queue_time {
					stats["register"].max_queue_time = time_in_que_register
				}
				stats["register"].total_queue_time += time_in_que_register

				time.Sleep(rnd_handle_time * time_mode)
				mu.Unlock()
				//printVehicleExit(vehicle)
				wg.Done()

			}
		}(register, idx)
	}

	wg.Wait()
	printStats(stats)
}

func printGasstationInitInfo(stations []Station, registers []Register) {
	fuel_count := make(map[string]int)
	for _, station := range stations {
		fuel_count[station.fuel.name]++
	}

	fmt.Println(strings.Repeat("#", 70))
	fmt.Println("stations: ")
	printed := make(map[string]bool)
	for _, station := range stations {
		fuel := station.fuel.name
		if _, ok := printed[fuel]; !ok {
			fmt.Println("  ", fuel)
			fmt.Println("    count: ", fuel_count[fuel])
			fmt.Println("    serve_time_min: ", station.serve_time_min.String())
			fmt.Println("    serve_time_max: ", station.serve_time_max.String())
			printed[fuel] = true
		}
	}

	fmt.Println("registers: ")
	fmt.Println("  count: ", len(registers))
	fmt.Println("  handle_time_min: ", registers[0].handle_time_min.String())
	fmt.Println("  handle_time_max: ", registers[0].handle_time_max.String())
	fmt.Printf("%20s\n", strings.Repeat("#", 70))

}
func printStats(stats map[string]*struct {
	total_car        int
	total_time       time.Duration
	total_queue_time time.Duration
	max_queue_time   time.Duration
}) {

	fmt.Println("stations: ")
	fuels := getSupportedFuels()
	for key, _ := range config {
		if slices.Contains(fuels, key) {
			fuel := key
			fmt.Printf("  %s:\n", fuel)
			fmt.Printf("    total_cars: %v\n", stats[fuel].total_car)
			fmt.Printf("    total_time: %.4fs\n", stats[fuel].total_time.Seconds())
			fmt.Printf("    avg_queue_time: %.4fs\n", stats[fuel].total_queue_time.Seconds()/float64(stats[fuel].total_car))
			fmt.Printf("    max_queue_time: %.4fs\n", stats[fuel].max_queue_time.Seconds())
		}
	}

	fmt.Println("registers: ")
	fmt.Printf("  total_cars: %v\n", stats["register"].total_car)
	fmt.Printf("  total_time: %.4fs\n", stats["register"].total_time.Seconds())
	fmt.Printf("  avg_queue_time: %.4fs\n", stats["register"].total_queue_time.Seconds()/float64(stats["register"].total_car))
	fmt.Printf("  max_queue_time: %.4fs\n", stats["register"].max_queue_time.Seconds())
	fmt.Printf("%20s\n", strings.Repeat("#", 70))
}

func printVehicleArrival(vehicle Vehicle, station_id string) {
	fmt.Printf("Car: %d (%s) arrived and joined the queue at the %s station\n", vehicle.id, vehicle.fuel.name, station_id)
}
func printVehicleStartRefuels(vehicle Vehicle, station_id string) {
	fmt.Printf("Car: %d (%s) started refueling at the %s station\n", vehicle.id, vehicle.fuel.name, station_id)
}
func printVehicleEndRefuels(vehicle Vehicle, station_id string, register_id int) {
	fmt.Printf("Car: %d (%s) ended refueling at the %s station and stands in %v queue to pay\n", vehicle.id, vehicle.fuel.name, station_id, register_id)
}
func printVehicleExit(vehicle Vehicle) {
	fmt.Printf("Car: %d (%s) leaved gas station\n", vehicle.id, vehicle.fuel.name)
}

func main() {
	simulateGasStation()
}
