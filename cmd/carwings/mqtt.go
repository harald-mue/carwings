package main

import (
	"errors"
	"fmt"
	"github.com/joeshaw/carwings"
	"log"
	"net/url"
	"os"
	"strconv"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func connect(clientId string, uri *url.URL) mqtt.Client {
	opts := createClientOptions(clientId, uri)
	client := mqtt.NewClient(opts)
	token := client.Connect()
	for !token.WaitTimeout(3 * time.Second) {
	}
	if err := token.Error(); err != nil {
		log.Fatal(err)
	}
	return client
}

func createClientOptions(clientId string, uri *url.URL) *mqtt.ClientOptions {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s", uri.Host))
	opts.SetUsername(uri.User.Username())
	password, _ := uri.User.Password()
	opts.SetPassword(password)
	opts.SetClientID(clientId)
	return opts
}

func listen(uri *url.URL, topic string, s *carwings.Session, cfg config) {
	client := connect("carwings", uri)
	// Subscribe for battery-update
	batteryUpdateTopic := fmt.Sprintf("%s/battery/update", topic)
	client.Subscribe(batteryUpdateTopic, 0, func(client mqtt.Client, msg mqtt.Message) {
		fmt.Printf("Incoming mqtt message -> [%s] %s\n", msg.Topic(), string(msg.Payload()))

		// Trigger battery update
		if string(msg.Payload()) == "1" {

			// Updtating battery status
			_, errUpdate := s.UpdateStatus()
			if errUpdate != nil {
				fmt.Fprintf(os.Stderr, "Error during carwings battery-update: %s\n", errUpdate)
			}

			// Retrieve battery status
			status, errStatus := s.BatteryStatus()
			if errStatus != nil {
				fmt.Fprintf(os.Stderr, "Error during carwings battery-update: %s\n", errUpdate)
			}

			// Publish battery status
			publishBatteryStatus(client, topic, status, cfg)

			// Send finished
			client.Publish(batteryUpdateTopic, 0, false, "0")
		}
	})
}

func publishBatteryStatus(client mqtt.Client, topic string, status carwings.BatteryStatus, cfg config) {
	retained := true
	client.Publish(fmt.Sprintf("%s/battery/timestamp", topic), 0, retained, status.Timestamp.String())
	client.Publish(fmt.Sprintf("%s/battery/stateofcharge", topic), 0, retained, strconv.Itoa(status.StateOfCharge))
	client.Publish(fmt.Sprintf("%s/battery/remaining", topic), 0, retained, strconv.Itoa(status.Remaining))
	client.Publish(fmt.Sprintf("%s/battery/capacity", topic), 0, retained, strconv.Itoa(status.Capacity))
	client.Publish(fmt.Sprintf("%s/battery/cruisingrangeacoff", topic), 0, retained, prettyUnits(cfg.units, status.CruisingRangeACOff))
	client.Publish(fmt.Sprintf("%s/battery/cruisingrangeacon", topic), 0, retained, prettyUnits(cfg.units, status.CruisingRangeACOn))
	client.Publish(fmt.Sprintf("%s/battery/remainingwh", topic), 0, retained, status.RemainingWH)
	client.Publish(fmt.Sprintf("%s/battery/pluginstate", topic), 0, retained, status.PluginState.String())
	client.Publish(fmt.Sprintf("%s/battery/chargingstatus", topic), 0, retained, status.ChargingStatus.String())
	client.Publish(fmt.Sprintf("%s/battery/timetofull/level1", topic), 0, retained, status.TimeToFull.Level1.String())
	client.Publish(fmt.Sprintf("%s/battery/timetofull/level2", topic), 0, retained, status.TimeToFull.Level2.String())
	client.Publish(fmt.Sprintf("%s/battery/timetofull/level2at6kw", topic), 0, retained, status.TimeToFull.Level2At6kW.String())
}

func runMqtt(s *carwings.Session, cfg config, args []string) error {
	uri, _ := url.Parse(fmt.Sprintf("tcp://%s:%s", cfg.mqttAddr, cfg.mqttPort))
	if len(cfg.mqttUsername) > 0 && len(cfg.mqttPassword) > 0 {
		uri.User = url.UserPassword(cfg.mqttUsername, cfg.mqttPassword)
	} else if len(cfg.mqttUsername) > 0 {
		uri.User = url.User(cfg.mqttUsername)
	}

	go listen(uri, cfg.mqttTopic, s, cfg)
	client := connect("pub", uri)

	if client.IsConnected() {
		exit := make(chan string)
		for {
			select {
			case <-exit:
				return nil
			}
		}
	} else {
		return errors.New("can't connect to mqtt-server")
	}
}
