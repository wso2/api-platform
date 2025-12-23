## Quick Start — API Platform Controller (ap)

This short guide explains how to use a prebuilt `ap` binary so you can run the CLI from any location.

- Install the binary: put the file on your `PATH`. For example:

	- macOS / Linux:

		```bash
		sudo mv ap /usr/local/bin/
		```

	- Windows:

		Place `ap.exe` in a folder that's included in your `%PATH%` (for example `%USERPROFILE%\bin`) or add its folder to your PATH.

- Gateway credentials: export `WSO2AP_GW_USERNAME` and `WSO2AP_GW_PASSWORD` before running controller commands.

- Policy Hub: if your environment overrides the default policy source, set the appropriate Policy Hub environment variable (for example `POLICY_HUB_URL`) to point to your policy hub.

After placing the binary on your `PATH` and exporting any required environment variables, you can run `ap` from any terminal.
