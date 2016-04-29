include $(GOROOT)/src/Make.inc

TARG=ninbot
GOFILES=\
	main.go\
	client.go\
	parse.go\

include $(GOROOT)/src/Make.cmd
