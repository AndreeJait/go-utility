package refactor

import (
	"context"
	"fmt"
	"github.com/AndreeJait/go-utility/loggerw"
	"os"
	"path/filepath"
	"strings"
)

func DoRefactor(beforePath, afterPath string) {
	log, err := loggerw.DefaultLog()
	if err != nil {
		panic(err)
	}

	err = filepath.Walk(".",
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				file, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				content := string(file)
				content = strings.ReplaceAll(content, beforePath, afterPath)

				err = os.WriteFile(path, []byte(content), 077)
				if err != nil {
					return err
				}
			}
			fmt.Println(path, info.Size())
			return nil
		})
	if err != nil {
		log.Println(context.Background(), err)
	}
}
