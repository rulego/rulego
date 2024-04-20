package service

import "examples/server/config"

func Setup(config config.Config) error {

	if s, err := NewUserService(config); err != nil {
		return err
	} else {
		UserServiceImpl = s
	}

	if s, err := NewUserRuleEngineServiceImpl(config); err != nil {
		return err
	} else {
		UserRuleEngineServiceImpl = s
	}

	if s, err := NewEventService(config); err != nil {
		return err
	} else {
		EventServiceImpl = s
	}

	return nil
}
