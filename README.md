Welcome to my funding tracker!

I regularly use funding rates on Hyperliquid to collect carry/basis from perpetual futures contracts. I noticed that there were no tools for querying historical funding data. 

This repo contains a python script that pulls funding data from the Hyperliquid API to Azure table storage and a cli tool written in go that is used to query this historical data. The cli app also contains a bash script that creates an executable for all architectures. 
