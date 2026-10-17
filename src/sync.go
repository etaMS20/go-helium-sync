package main

func runSync() error {
	path, err := heliumPath("Default/Bookmarks") // get the path to the Profile file
	if err != nil {
		return err
	}
	p, err := readHelium(path)
	if err != nil {
		return err
	}

	// Write the parsed profile back
	return writeAtomic(path, p)
}
