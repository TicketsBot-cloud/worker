package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/i18n"
)

func main() {
	dbclient.Connect()
	translations, err := dbclient.Client.Translations.GetAll()
	must(err)

	for lang, msgs := range translations {
		newMsgs := make(map[i18n.MessageId]string)
		for i, msg := range msgs {
			msgId := i18n.Messages[i]
			newMsgs[msgId] = msg
		}

		encoded, err := json.MarshalIndent(newMsgs, "", "	")
		must(err)

		path := fmt.Sprintf("./locale/%s.json", lang)
		must(ioutil.WriteFile(path, encoded, 0))
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
