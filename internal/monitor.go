package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

type Address string
type Addresses map[string][]Address

type config struct {
	testMnemonic string
	mainMnemonic string
	tftLimit     int
	botToken     string
	chatId       string
	intervalMins int
}

type monitor struct {
	env       config
	addresses Addresses
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

	Addresses, err := parseJson(jsonContent)
	if err != nil {
		return mon, err
	}

	mon.addresses = Addresses
	mon.env = env

	return mon, nil
}

// Start starting the monitoring service
func (m *monitor) Start() error {
	ticker := time.NewTicker(time.Duration(m.env.intervalMins) * time.Minute)

	for range ticker.C {
		for network, addressList := range m.addresses {

			mnemonic := m.env.mainMnemonic
			if network == "testnet" {
				mnemonic = m.env.testMnemonic
			}
			log.Debug().Msgf("mnemonics is set for %v", network)

			for _, address := range addressList {
				log.Debug().Msgf("monitoring for network %v, address %v", network, address)
				err := m.sendMessage(mnemonic, address)
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
func (m *monitor) sendMessage(mnemonic string, address Address) error {
	balance := m.getBalance(mnemonic, address)
	if balance >= m.env.tftLimit {
		return nil
	}

	url := fmt.Sprintf("%s/sendMessage", m.getTelegramUrl())
	body, _ := json.Marshal(map[string]string{
		"chat_id": m.env.chatId,
		"text":    fmt.Sprintf("account with address %v has balance = %v", address, balance),
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
func (m *monitor) getBalance(mnemonic string, address Address) int {
	log.Debug().Msgf("get balance for %v", address)
	return 100
}
