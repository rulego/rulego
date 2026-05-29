package services

// LocaleService 国际化服务接口
type LocaleService interface {
	Get(lang string) (interface{}, error)
	Save(lang string, data []byte) error
	List() ([]string, error)
}
