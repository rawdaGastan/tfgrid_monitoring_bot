package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	client "github.com/threefoldtech/substrate-client"
)

type Address string
type Network string

var (
	MainNetwork Network = "mainnet"
	TestNetwork Network = "testnet"
)

var SUBSTRATE_URLS = map[Network][]string{
	TestNetwork: {"wss://tfchain.test.grid.tf/ws"},
	MainNetwork: {"wss://tfchain.grid.tf/ws"},
}

type config struct {
	testMnemonic string
	mainMnemonic string
	tftLimit     int
	botToken     string
	chatId       string
	intervalMins int
}

type Wallets struct {
	mainnet []Address
	testnet []Address
}

type monitor struct {
	env       config
	wallets   Wallets
	substrate map[Network]client.Manager
}

// NewMonitor creates a new instance of monitor
func NewMonitor(envPath string, jsonPath string) (monitor, error) {
	mon := monitor{}

	envContent, err := readFile(envPath)
	if err != nil {
		return mon, err
	}

	env, err := parseEnv(string(envContent))
	if err != nil {
		return mon, err
	}

	jsonContent, err := readFile(jsonPath)
	if err != nil {
		return mon, err
	}

	addresses, err := parseJson(jsonContent)
	if err != nil {
		return mon, err
	}

	mon.wallets = addresses
	mon.env = env

	substrate := map[Network]client.Manager{}

	if len(mon.wallets.mainnet) != 0 {
		substrate[MainNetwork] = client.NewManager(SUBSTRATE_URLS[MainNetwork]...)
	}
	if len(mon.wallets.testnet) != 0 {
		substrate[TestNetwork] = client.NewManager(SUBSTRATE_URLS[TestNetwork]...)
	}

	mon.substrate = substrate

	return mon, nil
}

// Start starting the monitoring service
func (m *monitor) Start() error {
	ticker := time.NewTicker(time.Duration(m.env.intervalMins) * time.Minute)

	for range ticker.C {
		for network, manager := range m.substrate {

			wallets := []Address{}
			switch network {
			case MainNetwork:
				wallets = m.wallets.mainnet
			case TestNetwork:
				wallets = m.wallets.testnet
			}

			for _, address := range wallets {
				log.Debug().Msgf("monitoring for network %v, address %v", network, address)
				err := m.sendMessage(manager, address)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// getTelegramUrl returns the telegram bot api url
func (m *monitor) getTelegramUrl() string {
	return fmt.Sprintf("https://api.telegram.org/bot%s", m.env.botToken)
}

// sendMessage sends a message with the balance to a telegram bot
// if it is less than the tft limit
func (m *monitor) sendMessage(manager client.Manager, address Address) error {
	balance, err := m.getBalance(manager, address)
	if err != nil {
		return err
	}

	if balance >= m.env.tftLimit {
		return nil
	}

	url := fmt.Sprintf("%s/sendMessage", m.getTelegramUrl())
	body, _ := json.Marshal(map[string]string{
		"chat_id": m.env.chatId,
		"text":    fmt.Sprintf("account with address:\n%v\nhas balance = %v", address, balance),
	})
	response, err := http.Post(
		url,
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return err
	}
	if response.StatusCode >= 400 {
		return errors.New("request send message failed")
	}

	defer response.Body.Close()
	return nil
}

// getBalance gets the balance in TFT for the address given
func (m *monitor) getBalance(manager client.Manager, address Address) (int, error) {
	log.Debug().Msgf("get balance for %v", address)

	con, err := manager.Substrate()
	if err != nil {
		return 0, err
	}
	defer con.Close()

	account, err := client.FromAddress(string(address))
	if err != nil {
		return 0, err
	}

	balance, err := con.GetBalance(account)
	if err != nil {
		return 0, err
	}

	return int(balance.Free.Int64()), nil
}