// dnsmonster implements a packet sniffer for DNS traffic. It can accept traffic from a pcap file or a live interface,
// and can be used to index and store thousands of queries per second. It aims to be scalable and easy to use, and help
// security teams to understand the details about an enterprise's DNS traffic. It does not aim to breach
// the privacy of the end-users, with the ability to mask source IP from 1 to 32 bits, making the data potentially untraceable.

package main

import (
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/mosajjal/dnsmonster/capture"
	"github.com/mosajjal/dnsmonster/types"
	"github.com/mosajjal/dnsmonster/util"
	"github.com/pkg/profile"
	log "github.com/sirupsen/logrus"
)

var clickhouseResultChannel = make(chan types.DNSResult, util.GeneralFlags.ResultChannelSize)
var kafkaResultChannel = make(chan types.DNSResult, util.GeneralFlags.ResultChannelSize)
var elasticResultChannel = make(chan types.DNSResult, util.GeneralFlags.ResultChannelSize)
var splunkResultChannel = make(chan types.DNSResult, util.GeneralFlags.ResultChannelSize)
var stdoutResultChannel = make(chan types.DNSResult, util.GeneralFlags.ResultChannelSize)
var fileResultChannel = make(chan types.DNSResult, util.GeneralFlags.ResultChannelSize)
var syslogResultChannel = make(chan types.DNSResult, util.GeneralFlags.ResultChannelSize)
var resultChannel = make(chan types.DNSResult, util.GeneralFlags.ResultChannelSize)

func main() {

	// process and handle flags
	util.ProcessFlags()

	// debug and profile options
	runtime.GOMAXPROCS(util.GeneralFlags.Gomaxprocs)
	if util.GeneralFlags.Cpuprofile != "" {

		defer profile.Start(profile.CPUProfile).Stop()
	}
	// Setup the memory profile if reuqested
	if util.GeneralFlags.Memprofile != "" {
		go func() {
			time.Sleep(120 * time.Second)
			log.Warn("Writing memory profile")
			f, err := os.Create(util.GeneralFlags.Memprofile)
			util.ErrorHandler(err)
			runtime.GC() // get up-to-date statistics

			err = pprof.Lookup("heap").WriteTo(f, 0)
			util.ErrorHandler(err)
			f.Close()
		}()
	}

	// Setup our output channels
	setupOutputs()

	// Start listening if we're using pcap or afpacket
	if util.CaptureFlags.DnstapSocket == "" {
		capturer := capture.NewDNSCapturer(capture.CaptureOptions{
			util.CaptureFlags.DevName,
			util.CaptureFlags.UseAfpacket,
			util.CaptureFlags.PcapFile,
			util.CaptureFlags.Filter,
			uint16(util.CaptureFlags.Port),
			util.GeneralFlags.GcTime,
			resultChannel,
			util.CaptureFlags.PacketHandlerCount,
			util.CaptureFlags.PacketChannelSize,
			util.GeneralFlags.TcpHandlerCount,
			util.GeneralFlags.TcpAssemblyChannelSize,
			util.GeneralFlags.TcpResultChannelSize,
			util.GeneralFlags.DefraggerChannelSize,
			util.GeneralFlags.DefraggerChannelReturnSize,
			util.CaptureFlags.NoEthernetframe,
		})

		capturer.Start()
		// Wait for the output to finish
		log.Info("Exiting")
		types.GlobalWaitingGroup.Wait()
	} else { // dnstap si totally different, hence only the result channel is being pushed to it
		capture.StartDNSTap(resultChannel)
	}
}