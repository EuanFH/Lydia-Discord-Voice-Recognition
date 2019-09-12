# Lydia: Discord Voice Recognition
Lydia was my honors project for univeristy. Its goal was to create a virtual assistant in a multi-user VOIP enviroment. Essientially a Google Home or Amazon Alexa like assistant. Discord was chosen as the VOIP service, since it is the most popular in use that easily allows multiple user voice chat and allows bots that can send and recieve voice communication.

The application has 4 phases. The first is to listen to all users currently connected to the call listening for the key phrase "Hey Lydia". This was done using CMU Pocket Sphinx, this was chosen since it was a C library with bindings to Go. Pocket Sphinx is not the best way to do this sort of key word listening with more modern solutions existing. But they are only described in academic papers with no open source implementations.

Once a user has uttered the key phrase the application will start listening for a command. This was done using Google Cloud Speech to Text, this was choosen because it is by far the most accurate speech to text service.

When the user has finished speaking their command Google Cloud will return the text reponse of what the user has said. This is then processed using Rasa NLU a Natural Language Processing Application which is communicated to over a REST API. There was no API implementation in Go so one was written for this project. Rasa figures out the intent and entities from the users command. Example if the user command is "Play fog horn" based on training data it can extrapolate that the users intent is to "play horn" with an entity of "horn type" with the value of "fog".

The information recieved from Rasa is then sent to an external application as well as information about the user that said the command. Continuing with the air horn example, the external application could then join to VOIP call and play the fog horn sound effect. The external application also has the option of sending a text response that is read out to the user using Google Text to Speech.
 
You can see a video of the application in action here: [Demonstration](https://drive.google.com/file/d/1g9Te5Zy4T8kyLmBo7SqsZ16jNvo0INzd/view?usp=sharing)
 
