package main

import (
	"bufio"
	"flag"
	"log"
	"os"
	"sync"

	"github.com/cheggaaa/pb/v3"
	"github.com/nwaples/rardecode"
)

var (
	rarfile     string
	dictionary  string
	concurrency int
)

func init() {
	flag.StringVar(&rarfile, "f", "", "The path to the RAR file to crack")
	flag.StringVar(&dictionary, "d", "", "The path to the dictionary file")
	flag.IntVar(&concurrency, "c", 100, "The number of concurrent goroutines to use")
}

func main() {
	flag.Parse()

	if rarfile == "" || dictionary == "" {
		println("Usage: " + os.Args[0] + " -f <rarfile> -d <dictionary>")
		os.Exit(1)
	}

	word := make(chan string, concurrency) // Буферизированный канал
	found := make(chan string, 1)          // Буферизированный канал для одного найденного пароля
	stop := make(chan struct{})            // Канал для сигнала остановки

	var wg sync.WaitGroup

	dictFile, err := os.Open(dictionary)
	if err != nil {
		log.Fatalln(err)
	}
	defer dictFile.Close()

	scanner := bufio.NewScanner(dictFile)

	// Подсчет количества строк в словаре для прогресс-бара
	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}
	_, _ = dictFile.Seek(0, 0) // Возвращаемся в начало файла после подсчета строк
	scanner = bufio.NewScanner(dictFile)

	// Создание прогресс-бара
	bar := pb.StartNew(lineCount)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rarCrackWorker(rarfile, word, found, stop, bar)
		}()
	}

	go func() {
		for scanner.Scan() {
			select {
			case <-stop:
				return
			default:
				pass := scanner.Text()
				word <- pass
				bar.Increment() // Увеличиваем прогресс-бар на 1
			}
		}
		close(word)
	}()

	go func() {
		wg.Wait()
		close(found)
	}()

	for {
		select {
		case f, ok := <-found:
			if ok {
				close(stop) // Отправляем сигнал остановки всем горутинам
				println("[+] Found password:", f)
				bar.Finish() // Завершаем прогресс-бар
				return
			} else {
				println("[-] Password not found")
				bar.Finish() // Завершаем прогресс-бар
				return
			}
		}
	}
}

func rarCrackWorker(rarfilePath string, word <-chan string, found chan<- string, stop <-chan struct{}, bar *pb.ProgressBar) {
	rarFile, err := os.Open(rarfilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer rarFile.Close()

	for {
		select {
		case <-stop: // Если получен сигнал остановки, завершаем горутину
			return
		case pass, ok := <-word:
			if !ok {
				return
			}
			//println("Trying password:", pass) // Выводим каждый пароль перед попыткой
			_, err := rarFile.Seek(0, 0)
			if err != nil {
				log.Fatal(err)
			}

			rdr, err := rardecode.NewReader(rarFile, pass)
			if err != nil {
				continue
			}

			_, err = rdr.Next()
			if err == nil {
				found <- pass
				return
			}
		}
	}
}
