# Task Manager

### **Greetings**

Thank you for taking the time to review my project. I built this application using Golang and Tarantool, technologies that I believe are robust, efficient, and well-suited for developing scalable backend services.

My main goal is to show my ability to design and implement Chat-Bots, integrate with external systems, and write clean, maintainable, and production-ready code.

## How to run?
- Clone this repository using  
`git clone https://github.com/ce-fello/Mattermost-Voting-Bot`
- CD to the project folder
- There is a ready **Dockerfile** and **docker-compose.yml** to run services. If you already have token
      for your bot - paste it in `.env` file and `main.go`, `main_test.go`.    
Use `sudo docker up --build` to start building the project. If you don't have such, please follow 
      manuals to start **Mattermost** and **PostgreSQL** locally to get it ([about bot accounts](https://developers.mattermost.com/integrate/reference/bot-accounts/),
      [deploying with docker by mattermost](https://docs.mattermost.com/install/install-docker.html), [about tokens](https://developers.mattermost.com/integrate/reference/personal-access-token/), [for kali linux](https://ipv6.rs/tutorial/Kali_Linux_Latest/Mattermost/)).
      After you successfully connected to **Mattermost**, copy bot token and paste it where  
      it is needed (your_mattermost_token). Then build the project. If there is any package needed to continue - just download it and restart building. Or you can just use `docker run` for all services to start and then do `go run main.go`
## Bot commands
- `/vote create "<question>" "<option 1>" "<option 2>" ["<other options>"]` to create a voting
- `/vote info <vote_id>` to get current results of a voting.
- `/vote end <vote_id>` to end a voting (works only for creators)
- `/vote delete <vote_id>` to delete a voting (works only for creators)

## How to run tests?  
   - Use `go test` command to run all tests from `main_test.go`

## **Final Notes**

Thank you again for reviewing my project and taking the time to go through this instruction. I hope you found it clear, helpful, and enjoyable. I have designed the codebase to be clean, maintainable, and easy to navigate, ensuring that reading and understanding it will be a seamless experience.

If you have any questions, feedback, or further requests, I would be delighted to discuss them. I am confident that my work demonstrates my commitment to quality and my passion for software development. Looking forward to hearing from you!
