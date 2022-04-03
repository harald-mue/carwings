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

func connect(clientId string, uri *url.URL, topic string, s *carwings.Session, cfg config) mqtt.Client {
	fmt.Printf("Client-Id: %s\n", clientId)
	opts := createClientOptions(clientId, uri, topic, s, cfg)
	client := mqtt.NewClient(opts)
	token := client.Connect()
	for !token.WaitTimeout(3 * time.Second) {
	}
	if err := token.Error(); err != nil {
		log.Fatal(err)
	}
	return client
}

func createClientOptions(clientId string, uri *url.URL, topic string, s *carwings.Session, cfg config) *mqtt.ClientOptions {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s", uri.Host))
	opts.SetUsername(uri.User.Username())
	password, _ := uri.User.Password()
	opts.SetPassword(password)
	opts.SetClientID(clientId)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetKeepAlive(30 * time.Second)

	//set OnConnect handler as anonymous function
	//after connected, subscribe to topic
	opts.OnConnect = func(c mqtt.Client) {
		// Subscribe for battery-update
		batteryUpdateTopic := fmt.Sprintf("%s/battery/update", topic)
		fmt.Printf("Client connected, subscribing to: %s\n", batteryUpdateTopic)
		c.Subscribe(batteryUpdateTopic, 0, func(client mqtt.Client, msg mqtt.Message) {
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

		// Subscribe for daily-stats
		dailyUpdateTopic := fmt.Sprintf("%s/daily", topic)
		fmt.Printf("Client connected, subscribing to: %s\n", dailyUpdateTopic)
		c.Subscribe(dailyUpdateTopic, 0, func(client mqtt.Client, msg mqtt.Message) {
			fmt.Printf("Incoming mqtt message -> [%s] %s\n", msg.Topic(), string(msg.Payload()))

			// Trigger battery update
			if string(msg.Payload()) == "1" {

				// Updtating battery status
				stats, errUpdate := s.GetDailyStatistics(time.Now().Local())
				if errUpdate != nil {
					fmt.Fprintf(os.Stderr, "Error during carwings daily: %s\n", errUpdate)
				}

				// Publish battery status
				publishDailyStatus(client, topic, stats, cfg)

				// Send finished
				client.Publish(dailyUpdateTopic, 0, false, "0")
			}
		})
	}
	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		fmt.Fprintf(os.Stderr, "Connection to mqtt-server lost: %s\n", err)
	}
	return opts
}

func publishBatteryStatus(client mqtt.Client, topic string, status carwings.BatteryStatus, cfg config) {
	retained := true
	client.Publish(fmt.Sprintf("%s/battery/timestamp", topic), 0, retained, format(status.Timestamp))
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

func publishDailyStatus(client mqtt.Client, topic string, stats carwings.DailyStatistics, cfg config) {
	retained := false
	client.Publish(fmt.Sprintf("%s/daily/date", topic), 0, retained, format(stats.TargetDate))
	client.Publish(fmt.Sprintf("%s/daily/efficiency", topic), 0, retained, fmt.Sprintf("%.2f", stats.Efficiency))
	client.Publish(fmt.Sprintf("%s/daily/efficiencylevel", topic), 0, retained, strconv.Itoa(stats.EfficiencyLevel))
	client.Publish(fmt.Sprintf("%s/daily/efficiencyscale", topic), 0, retained, stats.EfficiencyScale)
	client.Publish(fmt.Sprintf("%s/daily/powerconsumedmotorlevel", topic), 0, retained, strconv.Itoa(stats.PowerConsumedMotorLevel))
	client.Publish(fmt.Sprintf("%s/daily/powerconsumedauxlevel", topic), 0, retained, strconv.Itoa(stats.PowerConsumedAUXLevel))
	client.Publish(fmt.Sprintf("%s/daily/powerregenerationlevel", topic), 0, retained, strconv.Itoa(stats.PowerRegenerationLevel))
	client.Publish(fmt.Sprintf("%s/daily/powerconsumedmotor", topic), 0, retained, fmt.Sprintf("%.2f", stats.PowerConsumedMotor))
	client.Publish(fmt.Sprintf("%s/daily/powerconsumedaux", topic), 0, retained, fmt.Sprintf("%.2f", stats.PowerConsumedAUX))
	client.Publish(fmt.Sprintf("%s/daily/powerregeneration", topic), 0, retained, fmt.Sprintf("%.2f", stats.PowerRegeneration))
}

/**
DateTimeType objects are parsed using Java's SimpleDateFormat.parse() in OpenHab using the pattern:
yyyy-MM-dd'T'HH:mm:ss
*/
func format(t time.Time) string {
	ret := fmt.Sprintf("%d-%02d-%02dT%02d:%02d:%02d",
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second())
	return ret
}

func runMqtt(s *carwings.Session, cfg config, args []string) error {
	uri, _ := url.Parse(fmt.Sprintf("tcp://%s:%s", cfg.mqttAddr, cfg.mqttPort))
	if len(cfg.mqttUsername) > 0 && len(cfg.mqttPassword) > 0 {
		uri.User = url.UserPassword(cfg.mqttUsername, cfg.mqttPassword)
	} else if len(cfg.mqttUsername) > 0 {
		uri.User = url.User(cfg.mqttUsername)
	}
	clientId := "carwings-" + fmt.Sprint(time.Now().Unix())
	client := connect(clientId, uri, cfg.mqttTopic, s, cfg)

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
