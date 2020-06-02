if [ "$USER" == "joey" ]; then
    #export RUN_ENV="fed1"
    sh ./config/env.sh "fed1"
    ./federation
fi
