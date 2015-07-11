package com.brotherlogic.recordcollection;

import com.brotherlogic.discogs.backend.CollectionBackend;
import com.brotherlogic.discogs.backend.UserBackend;
import com.brotherlogic.discogs.backend.WebCollectionBackend;
import com.brotherlogic.discogs.backend.WebUserBackend;

import org.scribe.model.OAuthRequest;
import org.scribe.model.Token;
import org.scribe.model.Verb;

public class DiscogsToken extends Token{

    private DiscogsService service;
    
    public DiscogsToken(String token, String secret, DiscogsService serv) {
        super(token, secret);
        service = serv;
    }

    public DiscogsToken(Token t, DiscogsService serv) {
        this(t.getToken(), t.getSecret(), serv);
    }

    public UserBackend getUserBackend(RequestBuilder builder) {
        return new WebUserBackend(new ScribeRetriever(this, service, builder));
    }

    public CollectionBackend getCollectionBackend(RequestBuilder builder) {
        return new WebCollectionBackend(new ScribeRetriever(this, service, builder));
    }
}
