/*
 * If you get an error and can't ping things, do `sudo sysctl -w net.ipv4.ping_group_range="0 65535"`
 * and run the program again.
 */

package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/parkerottaway/PingCLI/colors"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	TIMEOUT_SECS = 5
)

// Main function.
func main() {

	var conn *icmp.PacketConn
	var msg icmp.Message
	var success int = 0
	var sent int = 0
	var totalDuration time.Duration = 0
	defer conn.Close() // Close on panic hit.

	// Verify an argument was provided, exit if one was not.
	if len(os.Args) != 2 {
		fmt.Fprint(os.Stderr, colors.FG_RED, "PingCLI requires at least one input argument.\n", colors.RESET)
		os.Exit(0)
	}

	// Get the IP if hostname provided.
	ip, err := net.ResolveIPAddr("ip", os.Args[1])

	// Fail if an incorrect input provided.
	if err != nil {
		fmt.Println(err)
		os.Exit(0)
	}

	fmt.Print(colors.FG_GREEN, "Pinging ", ip.IP.String(), ":\n", colors.RESET)

	// Check if IP is IPv4 or v6.
	if ip.IP.To4() == nil { // Is v6
		conn, err = icmp.ListenPacket("udp6", "fe80::1%en0")

		if err != nil {
			fmt.Println("ListenPacket IPv6 error: ", err.Error())
		}

		msg = icmp.Message{
			Type: ipv6.ICMPTypeExtendedEchoRequest,
			Code: 0, // Code for echo reply.
			Body: &icmp.Echo{
				ID:   os.Getpid() & 0xffff,
				Seq:  1,
				Data: []byte("PING"),
			},
		}
	} else { // Is v4.
		conn, err = icmp.ListenPacket("ip4:icmp", "0.0.0.0")

		if err != nil {
			fmt.Println("ListenPacket IPv4 error: ", err.Error())
		}

		msg = icmp.Message{
			Type: ipv4.ICMPTypeEcho,
			Code: 0, // Code for echo reply.
			Body: &icmp.Echo{
				ID:   os.Getpid() & 0xffff,
				Seq:  1,
				Data: []byte("PING"),
			},
		}
	}

	message, err := msg.Marshal(nil) // Generate the checksum (if IPv4) and return message binary encoded.

	if err != nil {
		panic(err)
	}

	// Handle the SIGINT to calculate averages and loss percentage.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	// Information received from echo request.
	receiveChan := make(chan time.Duration)

	// Goroutine to ping website.
	go func(rChan chan time.Duration, c *icmp.PacketConn, ipaddr *net.IPAddr, m []byte) {
		for {
			startTime := time.Now() // Get current time.

			if _, err := c.WriteTo(m, ipaddr); err != nil { // Send packet.
				fmt.Println("Packet could not be sent...")
				fmt.Println(err.Error())
			}

			readBuffer := make([]byte, 1500)    // Create buffer with MTU.
			_, _, err := c.ReadFrom(readBuffer) // Read from the read buffer.

			// Check if there was an error when reading from connection.
			if err != nil {
				fmt.Println("There was an error receiving the packet...")
			}

			rChan <- time.Since(startTime) // Send the duration.
			time.Sleep(1 * time.Second)    // Wait for 1 second.
		}
	}(receiveChan, conn, ip, message)

	// Infinite loop
	for {

		// Wait for timeout or echo request.
		select {
		case <-time.After(TIMEOUT_SECS * time.Second): // Timeout.
			fmt.Println("Packet timed out...")
			sent++ // Increase total.

		case <-sigChan: // Catch SIGINT Signal.
			fmt.Print("\n\nReport:")
			fmt.Print(colors.FG_CYAN, "\nPackets sent:\t\t", sent, "\n", colors.RESET)
			fmt.Print(colors.FG_MAGENTA, "Packets received:\t", success, "\n", colors.RESET)

			// Calculate and print packet loss.
			rate := 100.0 * float32(sent-success) / float32(sent)
			if rate >= -1.0 && rate <= 33.3 { // Best.
				fmt.Print(colors.FG_GREEN, "Packet loss:\t\t", rate, "%\n", colors.RESET)
			} else if rate > 33.3 && rate <= 66.7 { // OK.
				fmt.Print(colors.FG_YELLOW, "Packet loss: ", rate, "%\n", colors.RESET)
			} else { // Worst.
				fmt.Print(colors.FG_RED, "Packet loss: ", rate, "%\n", colors.RESET)
			}
			// Print the averate RTT.
			fmt.Print("Average RTT:\t\t", time.Duration(int64(totalDuration)/int64(sent)), "\n")
			os.Exit(0)

		case input := <-receiveChan: // Ping is completed and duration is returned.
			// TODO Logic for measuring packet loss and average RTT.
			fmt.Println("RTT: ", input)
			sent++    // Increase total.
			success++ // Increase successful pings.
			totalDuration += input
		}
	}

}
