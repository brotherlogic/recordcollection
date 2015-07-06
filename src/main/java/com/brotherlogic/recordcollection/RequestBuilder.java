package com.brotherlogic.recordcollection;

import org.scribe.model.OAuthRequest;
import org.scribe.model.Verb;

public interface RequestBuilder {
    OAuthRequest buildRequest(Verb verb, String url);
}
