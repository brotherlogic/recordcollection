package com.brotherlogic.recordcollection;

import org.mockito.Mockito;

import org.junit.Assert;
import org.junit.Test;

import org.scribe.model.OAuthRequest;
import org.scribe.model.Response;
import org.scribe.model.Token;
import org.scribe.model.Verb;

public class ScribeRetrieverTest {

    @Test
    public void testScribeRetrieve() throws Exception {
        Response mResponse = Mockito.mock(Response.class);
        Mockito.when(mResponse.getBody()).thenReturn("testresponse");
        Token token = Mockito.mock(Token.class);
        DiscogsService service = Mockito.mock(DiscogsService.class);
        final OAuthRequest mRequest = Mockito.mock(OAuthRequest.class);
        Mockito.when(mRequest.send()).thenReturn(mResponse);
        
        ScribeRetriever retriever = new ScribeRetriever(token, service, new RequestBuilder() {
                public OAuthRequest buildRequest(Verb v, String url) {
                    return mRequest;
                }
            });
        String response = retriever.get("/testpart/");
        Assert.assertEquals("testresponse",response);
    }
    
}
