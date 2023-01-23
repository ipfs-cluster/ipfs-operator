########################################
# logs a message to stdout.
# Globals:
#		None.
# Arguments:
# 	A list of strings to print out.
########################################
log() {
	input="${*}"
	echo '['$(date)"]: ${input}\n"
}