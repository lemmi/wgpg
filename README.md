# WireGuard PlayGround

This tool aims to make the first contact with WireGuard even easier with a group people.   

## What's included

* Small webpage with instructions and links to get WireGuard installed an set up
* Api to register a public key (HTML Form)
* Adding peers to the server's WireGuard interface
* Config generation for the peers' public keys
* Display of all peers joined in the Network

## TODOs or rather "Wishlist":

* [ ] Include all web assets within the binary (go generate)
* [ ] Dedicated api endpoint with html output for browsers 
* [ ] Add config file with reasonable defaults
* [x] Translations / Multilang support
	* [x] de - mostly done, typos
	* [x] en
	* [ ] PRs welcome
* [ ] Eliminate shelling out to `wg` (just out of curiousity)
* [ ] Investigate if parts can run without root
	* Maybe split the webserver from the interface configuration
	* Use non-root user, but add CAP_NET_ADMIN
* [ ] Optionally include request ip as endpoint for peers
* [ ] Ipv6
* [ ] Better IP management
	* Currently every new PublicKey gets the next free IP until the the subnet is exhausted
	* No dead peer removal
	* No IP reuse
* [ ] Factor out wg-quick config file format into a library
* [ ] Add script to make client configs fully automatic (like https://www.wireguard.com/quickstart/#demo-server)
* [ ] Add License
* [x] Untangle the `net.IP` / `*net.IPNet` mess
