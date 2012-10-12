package main

func errStr(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}
