# 🦖 TestZilla 

TestZilla is a test  & benchmark framework, written in Golang, initially forked from [Plow](https://github.com/six-ddc/plow). TestZilla is under development.

 - [x] Generate 1000,000 RPS in your Home-Made Lab.

# Demo
![TestZilla Demo](demo.gif)

# About 

TestZilla is a distributed solution for stress test and benchmarks on Application Services, APIs, Web Servers and Network Protocols. TestZilla is currently under development, and its initial version is focused on Web Application Services and REST API. In TestZilla's development, we are trying to provide you with a home-based and cost-effective solution for setting up a benchmark lab. The goal is to create-N * 1000,000 RPS at your homemade lab. Currently, there are many open source solutions for stress testing and benchmarking in the open source community, thanks to all of them. We are trying to provide all the functional features of these tools in the form of a unique solution.

# Component & Modules
The TestZilla architecture is monolithic.
![TestZilla Internal](TestZilla.png)


# Compile & Usage
Follow the steps below to compile and run TestZilla:

``
make clean
``

then

``
make build
``

You can run TestZilla in one of the following modes:

1. Server: TestZilla as a master (Test Management Server)
2. Agent:  TestZilla as a agent (Test Node)
   3. Standalone: Run node in single mode
   4. Distributed: Run node in distributed mode 
 # Web Dashboard

![web dashboard](screen.png)



