package com.brotherlogic.recordcollection.integration;

import org.apache.log4j.Level;
import org.apache.log4j.Logger;

import com.brotherlogic.recordcollection.ContextListener;
import com.brotherlogic.recordcollection.RcSystem;
import com.brotherlogic.recordcollection.TestingSystem;

import org.junit.Assert;
import org.junit.Test;

import org.scribe.model.Token;

public class LoginTestIT extends BaseIntegrationTestIT {

  private Logger logger = Logger.getLogger(getClass());
  
  @Test
  public void testStoreAndRetrieveLoginDetails() {
    //Login
    RcSystem system = new TestingSystem();
    system.getStorage().storeToken(new Token("testkey","testsecret"));

    //Check that the login details are maintained        
    Token rToken = new TestingSystem().getStorage().getToken("testkey");
    logger.log(Level.DEBUG,"Returned: " + rToken);
    Assert.assertEquals("testsecret",rToken.getSecret());
  }
  
}
