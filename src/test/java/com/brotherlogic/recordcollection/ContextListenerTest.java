package com.brotherlogic.recordcollection;

import java.util.Map;

import javax.servlet.ServletContext;
import javax.servlet.ServletContextEvent;

import org.junit.Assert;
import org.junit.Test;

import org.mockito.Mockito;

public class ContextListenerTest {

    @Test
    public void testDestroyed() {
        //Basically this should not fail
        ContextListener listener = new ContextListener();
        ServletContextEvent ev = Mockito.mock(ServletContextEvent.class);
        listener.contextDestroyed(ev);
    }
    
    @Test
    public void testInitialisation() {
        //Setup some dummy values
        System.setProperty("discogs-key","madeupkey");
        System.setProperty("discogs-secret","madeupsecret");
        
        //This will break up the logging process
        ContextListener listener = new ContextListener();
        ServletContextEvent ev = Mockito.mock(ServletContextEvent.class);
        ServletContext context = Mockito.mock(ServletContext.class);
        Mockito.when(ev.getServletContext()).thenReturn(context);
        listener.contextInitialized(ev);

        Mockito.verify(context).setAttribute(Mockito.eq("config"), Mockito.any(Config.class));
        Mockito.verify(context).setAttribute(Mockito.eq("token_map"), Mockito.any(Map.class));
        Mockito.verify(context).setAttribute(Mockito.eq("auth_tokens"), Mockito.any(Map.class));
    }
}
