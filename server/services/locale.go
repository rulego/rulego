package services

// LocaleService international service interface
type LocaleService interface {
	Get(lang string) (interface{}, error)
	Save(lang string, data []byte) error
	List() ([]string, error)
}
