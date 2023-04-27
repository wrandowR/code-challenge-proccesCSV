package interactor

import (
	"encoding/csv"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/ansel1/merry/v2"
	"github.com/sirupsen/logrus"
	"github.com/wrandowR/code-challenge/config"
	"github.com/wrandowR/code-challenge/domain/model"
	repository "github.com/wrandowR/code-challenge/usecase/repository"
	"github.com/wrandowR/code-challenge/usecase/service"
)

// fileProcessor is a service that process a csv file
type fileProcessor struct {
	DataStore   repository.Transactions
	EmailSender service.EmailSender
}

func NewFileProcessor(dataStorage repository.Transactions, emailSender service.EmailSender) *fileProcessor {
	return &fileProcessor{
		DataStore:   dataStorage,
		EmailSender: emailSender,
	}
}

func (s *fileProcessor) ProccesFile(dir string) error {

	// Open the file
	file, err := os.Open(dir)
	if err != nil {
		return merry.Wrap(err)
	}
	//defer file.Close()

	csvReader := csv.NewReader(file)

	records, err := csvReader.ReadAll()
	if err != nil {
		return merry.Wrap(err)
	}

	//delete header
	records = records[1:]

	var totalBalance float64
	var totalCreditTransactions float64
	var totalDebitTransactions float64
	var AverageCreditAmountData []float64
	var AverageDebitAmountData []float64

	// Create worker pool
	var wg sync.WaitGroup
	jobs := make(chan []string, len(records))
	results := make(chan model.Transaction, len(records))

	for i := 0; i < config.MaxGoroutines(); i++ {
		wg.Add(1)
		go worker(jobs, results, &wg)
	}

	// Send jobs to workers
	for _, record := range records {
		jobs <- record
	}
	close(jobs)

	monthMap := make(map[string]int)

	// Collect results from workers

	for i := 0; i < len(records); i++ {
		result := <-results

		getTransactionsPerMonth(monthMap, result.Date)

		if result.IsNegative {
			totalBalance -= result.Amount
			totalDebitTransactions += result.Amount
			AverageDebitAmountData = append(AverageDebitAmountData, result.Amount)
			continue
		}

		totalBalance += result.Amount
		totalCreditTransactions += result.Amount
		AverageCreditAmountData = append(AverageCreditAmountData, result.Amount)

	}

	wg.Wait()

	TransactionInAMounth := []model.TransactionInAMounth{}
	for key, value := range monthMap {
		transactions := model.TransactionInAMounth{
			Month: key,
			Total: float64(value),
		}
		TransactionInAMounth = append(TransactionInAMounth, transactions)
	}

	transactionEmailData := model.TransactionEmail{
		TotalBalance:        math.Round(totalBalance*100) / 100,
		Transactions:        TransactionInAMounth,
		AverageDebitAmount:  average(AverageDebitAmountData),
		AverageCreditAmount: average(AverageCreditAmountData),
	}

	if err := s.EmailSender.SendEmail("test", &transactionEmailData); err != nil {
		return merry.Wrap(err)
	}
	logrus.Info("Email sent")
	return nil
}

func worker(jobs <-chan []string, results chan<- model.Transaction, wg *sync.WaitGroup) {
	defer wg.Done()

	for job := range jobs {

		cleanTransactionAmount, err := cleanAndParseTransaction(job[2])
		if err != nil {
			log.Fatal(err)
		}
		ok := isNegative(job[2])

		/*	var cleantAmount float64 = cleanTransactionAmount
			if ok {
				cleantAmount = cleanTransactionAmount * -1
			}
		*/
		/*
			//guardar en base de datos
			transactionResult, err := dataStore.SaveTransaction(&model.Transaction{
				IsNegative: ok,
				Amount:     cleantAmount,
				Date:       job[1],
			})
		*/
		//validar esto aca no estoy seguro
		if err != nil {
			log.Fatal(err)
		}

		//fmt.Println(transactionResult, "RESULTADO TRANSACCION")

		results <- model.Transaction{
			IsNegative: ok,
			Amount:     cleanTransactionAmount,
			Date:       getMonth(job[1]),
		}
	}
}

// funcion que tetorna si un string de numer so es negativo o positivoas
func isNegative(s string) bool {
	return s[0] == '-'
}

func average(numbers []float64) float64 {
	var sum float64
	if len(numbers) == 0 {
		return sum
	}
	for _, num := range numbers {
		sum += num
	}
	return sum / float64(len(numbers))
}

func cleanAndParseTransaction(transaction string) (float64, error) {
	//remove the non-numeric characters from the string
	transaction = strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || r == '.' {
			return r
		}
		return -1
	}, transaction)

	//convert the string to a decimal number
	value, err := strconv.ParseFloat(transaction, 64)
	if err != nil {
		return 0, err
	}
	return value, nil
}

func getMonth(date string) string {
	parts := strings.Split(date, "/")
	month, _ := strconv.Atoi(parts[0])
	months := []string{
		"January",
		"February",
		"March",
		"April",
		"May",
		"June",
		"July",
		"August",
		"September",
		"October",
		"November",
		"December"}

	return months[month-1]
}

func getTransactionsPerMonth(monthMap map[string]int, month string) map[string]int {

	if _, ok := monthMap[month]; ok {
		monthMap[month]++
		return monthMap
	}
	monthMap[month] = 1

	return monthMap
}
