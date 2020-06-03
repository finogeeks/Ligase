if [ "$USER" == "joey" ]; then
    #export RUN_ENV="fed2"
    sh ./config/env.sh "fed2"
    ./federation
fi
