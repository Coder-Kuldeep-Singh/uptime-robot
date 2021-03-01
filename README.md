Uptimerobots' work is to monitor the recommendation server in every 5 minutes if it got server is
down so it will gonna send the email to the me and Felipe so we can check the why server is down(reason)

Q-> How it will gonna work
A -> 1. IT reads the txt file which have the url to check
     2. Sends the request to the first url if it gets the status code above than 399
        so we sends the more requests in 30 seconds if all of those urls' status code is 
        above than 399 so we send the email alert message to the used email address