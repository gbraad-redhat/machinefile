package main

// Define the categories and their flags in desired order
var categories = []flagCategory{
	{
		name: "Runner Selection",
		flags: []string{
			"local",
			"l",
			"podman",
			"p",
			"ssh",
			"s",
		},
	},
	{
		name: "File Options",
		flags: []string{
			"file",
			"f",
			"context",
			"c",
		},
	},
	{
		name: "SSH Options",
		flags: []string{
			"host",
			"H",
			"user",
			"u",
			"key",
			"password",
			"ask-password",
			"port",
		},
	},
	{
		name: "Podman Options",
		flags: []string{
			"name",
			"n",
			"connection",
			"podman-binary",
		},
	},
	{
		name: "Other Options",
		flags: []string{
			"stdin",
			"arg",
			"help",
		},
	},
}
