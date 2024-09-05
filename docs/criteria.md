This item gives 10 points. Without such a file, the solution is not checked.
0. Necessary requirements:
- There is a Readme document that describes how to start the system and how to use it.
      - [README.md ](../README.md )
- It can be docker-compose, makefile, detailed instructions - to your taste
     - [docker-compose](../docker-compose.yml)
- If you provide only the http api, then the Readme describes examples of requests using curl-a or in any other understandable way
     - [postman](Distibuted%20calculation.postman_collection.json) 
   - the examples are complete and it is clear how to run them
     - In postman and [README.md ](../README.md )
   

1. The program starts and all the examples with the calculation of arithmetic expressions work correctly - 10 points
    - It works
2. The program is started and arbitrary examples are executed with the calculation of arithmetic expressions - 10 points
    - It works
3. You can restart any component of the system and the system will process the restart correctly (the results are saved, the system continues to work) - 10 points
    - When the orchestrator is running, it sends all expressions with the status 0 for calculation.
4. The system provides a graphical interface for calculating arithmetic expressions - 10 points
    - No, it's not
5. Monitoring of workers has been implemented - 20 points
    - Yes
6. Implemented an interface for the monitoring of workers - 10 points
    - No, it's not
7. You understand the code base and the structure of the project - 10 points (this is a subjective criterion, but the simpler your solution, the better).
The supervisor at this point honestly answers the question: "Can I make a pull request to the project without a nervous breakdown"

1. The system has documentation with diagrams that clearly answers the question: "How does it all work" - 10 points
    - ![image](system%20scheme.svg)
2. The expression must be able to be executed by different agents - 10 points
    - Implemented, since agents accept parts of the expression, not the whole of them.